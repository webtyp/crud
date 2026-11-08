---
PLAN: "feat!: crud persists tables — Registry.Table with Store/Tx, per-record hooks, composite keys, public actions; Create/Update/Delete leave Registry"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 7346834124478764260
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work. Commit after every stage.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.

## Executor notes
`go doc webtyp.com/model Allower` fails. Stopped execution as per PLAN.md instructions.

# Plan: `crud` persists a table; a module declares it and writes only domain rules

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
go get webtyp.com/model@latest webtyp.com/storage@v0.1.18 webtyp.com/time@latest
```
`model@latest` must contain `model.Allower` and `Definition.Allow` (plan A6.1); if `go doc webtyp.com/model Allower` fails,
stop and say so in the PR. No `replace` in `go.mod`. `crud` must **not** import `webtyp.com/orm` (orm implements `crud.Store`
in the next plan; `grep -rn "webtyp.com/orm" .` → empty).

## Why

Today `Registry.Create/Read/Update/Delete` hand each module a raw list, and every module re-writes the same mechanics:
id, tenant, timestamps, schema validation, a loop inside a transaction, tenant-scoped update/delete, partial update by
`fields`. Measured in `veltylabs/modules/editorial/crud.go` (226 lines): `UpdatePost` writes the whole record ignoring
`fields` (it blanks `author_id` and `title`), reads outside the transaction, repeats the schema validation; four
operations that must not exist are mounted and answer `"not supported"`. The tenant filter written by hand in every
function is what produced the IDOR in `patient_directory`. Decisions D19 and D20 of
`webtyp/docs/ARCHITECTURE_MASTER_PLAN.md`: `crud` persists; the module declares what a table allows and writes
per-record hooks. `ormc` (next plan) generates the typed `Mount<Entity>` that builds a `crud.Table`.

## Ecosystem search

- `crud` today: `create.go`, `update.go`, `delete.go`, `read.go`, `validate.go` (decode, `validateCreateBatch`,
  `validateUpdateFields`, `validateIDs`, `validateReadArgs`, `ServerOnly`) — **reused** by the new engine.
- `orm.DB` (`Tx`, `Create`, `UpdateFields`, `Delete`, `Query`/`ReadAll`) — the methods `crud.Tx` mirrors by name, so
  `orm` implements it with a thin adapter (plan A6.3). `crud` declares the contract (api-harness rule 9).
- `storage.Condition` (`storage v0.1.18`) — reused for conditions. **It has no grouping**: conditions are evaluated left
  to right (`storage/mem/mem.go:311`); an `OR` would turn `tenant = X AND a OR b` into a cross-tenant read. Hence rule R1 below.

## Design gate

1. **Prior art:** Django `ModelAdmin.save_model`/`Model.clean` (https://docs.djangoproject.com/en/5.1/ref/contrib/admin/#django.contrib.admin.ModelAdmin.save_model),
   Rails callbacks (https://guides.rubyonrails.org/active_record_callbacks.html), Spring Data REST event handlers
   (https://docs.spring.io/spring-data/rest/reference/events.html). The framework persists; the application plugs rules
   per record. Same here, without generics: the typed hooks are produced by `ormc`.
2. **Names:** `Registry.Table`, `crud.Table`, `crud.Store`, `crud.Tx` with the words `orm` already uses (`ReadOne`,
   `ReadAll`, `Create`, `UpdateFields`, `Delete`, `Tx`). Hooks named by the action they guard (`Create`, `Update`, `Delete`),
   plus `Search`, `NewFilter`, `Committed`.
3. **Ledger:** concepts +3 (`Table`, `Store`, `Tx`) −4 (`Registry.Create/Update/Delete` + their adapters); lines at a module's
   call site −150 to −250 per module; ways to do the same thing −1 (no hand-written persistence path left for tables).
4. **Where:** `crud` already owns the operation contract and its validation.
5. **Deletes:** `Registry.Create`, `Registry.Update`, `Registry.Delete` (and their handlers' registry methods);
   `Route.Describe` for tables (descriptions come from the model and action).

## APIs used

| Symbol | Signature | Tag |
|---|---|---|
| `model.Allower` | `interface{ Allow() model.Action }` | A6.1 |
| `model.Action.Has` | `func (a Action) Has(want Action) bool` | `model` |
| `model.Field` | `.Name`, `.IsPK() bool`, `.ServerOnly`, `.Item model.ItemSlot` (`model.ItemLabel`) | `model` |
| `model.IDGenerator` | `interface{ NewID() string }` | `model` |
| `storage.Condition`, `storage.Eq/Gt/Like`, `Condition.Logic() string` (`"AND"`/`"OR"`) | | `storage v0.1.18` |
| `time.NowNano() int64` | `webtyp.com/time` | |

If a signature differs, stop and name the symbol in the PR; never adapt it by guessing.

## Stage 1 — contract (`contract.go`, `store.go` new)

```go
// contract.go
type Registry interface {
	// Table mounts the actions t.Model allows, persisted by t.Store. ormc's Mount<Entity> is its only caller.
	Table(t Table)
	// Read mounts a read of a model that is not a table (a report, a derived view), answered by read.
	Read(m model.ModuleNaming, newFilter func() model.Model, read func(c Caller, args ReadArgs, filter model.Model) (model.ModelSlice, error)) Route
}

// Table is one table a module offers. ormc builds it from typed hooks; a hook is nil when the module has no rule.
type Table struct {
	Model     model.Model            // a zero record; must implement model.Allower with Allow() != 0
	NewList   func() model.ModelSlice
	Store     Store
	Create    func(c Caller, tx Tx, rec model.Model) error                       // per record, before insert, inside the transaction
	Update    func(c Caller, tx Tx, old, next model.Model, fields []string) error // per id; old is the stored row, next carries only fields
	Delete    func(c Caller, tx Tx, rec model.Model) error                       // per id, before delete; rec is the stored row
	Search    func(c Caller, text string) ([]storage.Condition, error)          // replaces the default search; AND-only conditions
	NewFilter func() model.Model                                                 // typed filter; equality on its non-empty fields
	Committed func(c Caller, action model.Action, rec model.Model, fields []string) // after commit, once per record (events); fields = the updated fields on Update, nil otherwise
	MFA       model.Action                                                       // actions that require a second factor
}
```

```go
// store.go
// Query is what a read asks a Store for. Without Order, rows come ordered by primary key, ascending.
type Query struct {
	Conditions []storage.Condition // AND only (rule R1)
	Order      []storage.Order     // empty = primary key ascending; a hook reads e.g. by expiry date (storage.Asc("expires_at"))
	After      string              // rows with primary key > After; only valid with an empty Order (ErrAfterWithOrder) and a single-column key (ErrAfterCompositeKey)
	Limit      int64               // already capped
}

// Tx reads and writes one table's rows. A Store scoped to a tenant applies the tenant to every call.
type Tx interface {
	ReadOne(rec model.Model, id string) error                                          // an error of kind model.ErrorNotFound when absent
	ReadAll(newRow func() model.Model, q Query, onRow func(model.Model)) error
	Create(rec model.Model) error
	UpdateFields(rec model.Model, id string, fields []string) error
	Delete(rec model.Model, id string) error
}

// Store is the repository of one table or resource (Repository pattern): orm.TenantStore for a table of the module,
// or a Store the module writes over another service's API (D21).
type Store interface {
	Tx
	NewID() string
}

// Transactional is an optional Store capability: fn runs in one transaction and an error rolls everything back.
// A Store without it gets batches of one record only (ErrBatchNotAtomic): crud never promises atomicity that does not exist.
type Transactional interface {
	Atomic(fn func(tx Tx) error) error
}
```

Rule **R1** (documented on `Query` and enforced in `crud`): every condition passed to a `Store` has `Logic() == "AND"`.
`crud` checks the conditions a `Search` hook returns; one `"OR"` → `ErrAfterWithOrder model.Invalid = "crud: After pages by primary key; it cannot be combined with another Order"` and `ErrAfterCompositeKey model.Invalid = "crud: After pages by a single-column key; list this table whole (its screen uses view.Page{First: view.All})"` (checked by every Store).
`ErrOrCondition model.Invalid = "crud: a search
returned an OR condition; conditions are not grouped, so an OR could read other tenants' rows — return AND conditions only"`.

### Public actions and `Mount` (D23)

```go
// Public is one action the app opens to visitors without a session (reads and writes). Only the app lists them.
//
//	Public: []crud.Public{{Model: &abook.OnlineReservation{}, Actions: model.Create}}
type Public struct {
	Model   model.ModuleNaming
	Actions model.Action
}

func Mount(reg router.OperationRegistry, public []Public, m Module) error // was public []model.ModuleNaming
```
`registry.isPublic(name string, action model.Action) bool` replaces `isPublic(name)`; `Registry.Read` (wire models) checks
`isPublic(name, model.Read)`. A public action is mounted with `.Public()` instead of `.Requires(...)`; its hooks receive
`Caller{UserID: ""}`. Update every caller in this repo (`loopback.go`, tests).

### Composite primary keys (D22)

`table.go` keeps `pk []int` (indexes of every `IsPK()` field, schema order). The wire id of a record is `model.JoinKey(<pk values>...)`
(one part = the plain value). Helpers: `keyOf(rec) string` and `setKey(rec, id) error` (`model.SplitKey(id, len(pk))`; not ok →
`ErrInvalidID`). A key part is a `*string` or an `*int64` in `Pointers()` (an `int64` part is written and parsed as decimal text with
`webtyp.com/fmt`); any other kind → mount error `ErrKeyKind = "crud: a primary key field must be text or int64"`. Create: single text key
empty → `Store.NewID()`; composite key → every part required (empty/zero → field error on that field). **Any** key value, single or
composite, containing `model.KeySeparator` → `ErrKeyHasSeparator model.Invalid = "crud: a key value cannot contain /"`.
The idempotent-retry check, `ReadOne`, `UpdateFields`, `Delete` and `Committed` use `keyOf`/`setKey` instead of the single pk.

## Stage 2 — mount checks (`table.go` new)

`func (r *registry) Table(t Table)` fails the mount (stored in `r.err`, returned by `crud.Mount`, so the server does not
boot) with these errors (`mountError`):

| Condition | Error |
|---|---|
| `t.Model` nil or `t.NewList` nil | `ErrNilFunction` (existing) |
| `t.Model` not `model.Allower`, or `Allow() == 0` | `ErrNothingAllowed = "crud: Table for a model that allows nothing; set Allow in its model.Definition"` |
| `t.Store` nil | `ErrNoStore = "crud: Table without a Store; pass orm.TenantStore(db, tenantID, ids)"` |
| a hook set for an action not allowed (`Create` set and `!Allow.Has(model.Create)`, same for `Update`, `Delete`; `Search`/`NewFilter` set without `Read`) | `ErrHookWithoutAction = "crud: a rule is set for an action the model does not allow"` |
| no primary-key field in `t.Model.Schema()` | `ErrNoPrimaryKey = "crud: Table model has no primary key field"` |
| `t.Store` implements `ModelChecker` and `CheckModel(t.Model)` returns an error | that error (e.g. `orm.TenantStore` on a table without `tenant_id`) |

Add to `store.go`:
```go
// ModelChecker is an optional Store capability: Table calls it at mount so a wrong Store for a model stops the boot.
type ModelChecker interface {
	CheckModel(m model.Model) error
}
```

Then it registers, for each allowed action, the operation `Op(t.Model, action)` with `.Public()` when `isPublic(ModelName, action)`,
else `Requires(model.Resource(ModelName), action)`; `Accepts(t.Model)` for Create/Update; `RequiresMFA()` when `t.MFA.Has(action)`.

## Stage 3 — the engine (`table_create.go`, `table_update.go`, `table_delete.go`, `table_read.go`)

Helpers in `table.go`: `pkIndex(schema)` (index of the PK field), `setString(rec, i, v)` / `setInt(rec, i, v)` through
`rec.Pointers()[i].(*string)` / `(*int64)`; `stampIndex(schema, name)` for `created_at` / `updated_at`
(constants `FieldCreatedAt = "created_at"`, `FieldUpdatedAt = "updated_at"`; a field with that name must be `*int64` in
`Pointers()`, else mount error `ErrStampNotInt = "crud: created_at/updated_at must be an int64 nanosecond field"`).

Helper in `table.go`:
```go
// atomic runs fn in one transaction when the Store can; otherwise fn runs on the Store itself (batches of one, checked before).
func atomic(s Store, fn func(tx Tx) error) error {
	if t, ok := s.(Transactional); ok {
		return t.Atomic(fn)
	}
	return fn(s)
}
```
Before any write, a request with more than one record (Create) or more than one id (Update, Delete) on a Store that is not
`Transactional` → `ErrBatchNotAtomic model.Invalid = "crud: this resource saves one record at a time; send one record per request"`.

**Create** (handler): decode `createIn{list: t.NewList()}` → `validateCreateBatch(schema, list, tooMany)` (existing) → 
```go
err := atomic(t.Store, func(tx Tx) error {
	now := time.NowNano()
	for i := 0; i < list.Len(); i++ {
		rec := list.At(i).(model.Model)
		if id := *rec.Pointers()[pk].(*string); id == "" {
			setString(rec, pk, t.Store.NewID())
		} else {
			// Retry is idempotent: ids made in the browser (offline-first) may arrive twice. A row with this id already
			// stored (in this Store's scope) means this record was created before: skip it — no hook, no insert, no Committed.
			stored := t.NewList().Append().(model.Model)
			if err := tx.ReadOne(stored, id); err == nil {
				if err := tx.ReadOne(rec, id); err != nil { // the response carries what is stored, not what was sent
					return err
				}
				skipped[i] = true
				continue
			} else if model.KindOf(err) != model.ErrorNotFound {
				return err
			}
		}
		if created >= 0 { setInt(rec, created, now) }
		if updated >= 0 { setInt(rec, updated, now) }
		if t.Create != nil {
			if err := t.Create(caller, tx, rec); err != nil {
				return err
			}
		}
		if err := tx.Create(rec); err != nil {
			return err
		}
	}
	return nil
})
```
(`skipped := make([]bool, list.Len())` before `Atomic`.) Then, on success, `t.Committed(caller, model.Create, rec, nil)` for each record not skipped (if set), and encode the last record (existing behaviour). Add test `TestTable_CreateRetryIsIdempotent`: create `{id:"p1"}` twice → second call succeeds, one row, `Committed` called once.

**Update**: decode `UpdateArgs{Record: newRecord}` (newRecord = `t.NewList().Append().(model.Model)` each call) →
`validateIDs`, `validateUpdateFields` (existing) → `fields` plus `FieldUpdatedAt` when present and not already listed →
```go
err := atomic(t.Store, func(tx Tx) error {
	for _, id := range in.IDs {
		old := t.NewList().Append().(model.Model)
		if err := tx.ReadOne(old, id); err != nil {
			return err // not found stays not found: 404
		}
		next := in.Record
		setString(next, pk, id)
		if updated >= 0 { setInt(next, updated, time.NowNano()) }
		if t.Update != nil {
			if err := t.Update(caller, tx, old, next, in.Fields); err != nil {
				return err
			}
		}
		if err := tx.UpdateFields(next, id, fields); err != nil {
			return err
		}
	}
	return nil
})
```
After commit, per id: `setString(next, pk, id)` then `Committed(caller, model.Update, next, in.Fields)` (one `next` is shared by all ids; set its id before each call). A hook may change `next`, but only the columns in `fields` are written; to write another column of this or another row, the hook calls `tx.UpdateFields` itself (document this on `Table.Update`). Status 204 (existing).

**Delete**: decode `DeleteArgs` → `validateIDs` → in `atomic(t.Store, …)`: per id a fresh `old := t.NewList().Append().(model.Model)`, `ReadOne(old, id)`, hook `Delete(caller, tx, old)`, `tx.Delete(old, id)`, append `old` to `deleted`;
after commit `Committed(caller, model.Delete, row, nil)` for each row of `deleted`. Status 204.

**Read**: decode `ReadArgs` (with `Filter = t.NewFilter()` when set) → `validateReadArgs` (existing) →
- `ID != ""` → `ReadOne` into a new row → list of one (not found → 404).
- else conditions: `Search != ""` → `t.Search(caller, text)` when set (check R1), otherwise `storage.Like(<the ItemLabel field>, "%"+lower(text)+"%")`
  (the one field whose `Item == model.ItemLabel`; no such field → `ErrNoSearchField model.Invalid = "crud: this table has no searchable field"`);
  filter → `storage.Eq(field.Name, value)` for each filter field whose pointer holds a non-zero value (`*string != ""`,
  `*int64 != 0`, `*bool == true`; other kinds: skip and document);
- `ReadAll(newRow, Query{Conditions, After: args.After, Limit: Cap(args.Limit)}, append)`; encode the list (existing).

## Stage 4 — deletions

Delete `func (r *registry) Create`, `func (r *registry) Update`, `func (r *registry) Delete` (keep `CreateArgs`, `UpdateArgs`,
`DeleteArgs`, `createIn` — the wire types the engine and the clients use). `grep -rn "func (r \*registry) \(Create\|Update\|Delete\)(" .` → empty.
`Route.Describe` stays only on `Read` (wire models); tables get the description `"<Action> <ModelName>"` built in `table.go`.

## Stage 5 — conformance suite (`storetest/storetest.go` new) and tests

`package storetest` exports `func Run(t *testing.T, newStore func(t *testing.T) crud.Store)` — every `Store` implementation runs
it (orm in A6.3): ReadOne not found has kind `model.ErrorNotFound`; Create then ReadOne round-trips; UpdateFields changes only the
listed fields; ReadAll orders by id (or by `Order`) and honours After/Limit; After with a non-empty Order → `ErrAfterWithOrder`; when the Store is `Transactional`, Atomic rolls back on error; a Store rejects an OR condition.
It uses a test model defined in `storetest` (id, name with `Item: model.ItemLabel`, created_at, updated_at, `Allow` all).

`tests/` (rows in `docs/TESTS.md`) use an in-memory `Store` written in `tests/memstore_test.go` that passes `storetest.Run`, and
`crud.Loopback` with a module whose `MountOperations` calls `reg.Table(...)` directly (no ormc here):

| Test | Setup → call → expected |
|---|---|
| `TestTable_NothingAllowedFailsMount` | model with `Allow() == 0` → `crud.Mount` returns `ErrNothingAllowed` |
| `TestTable_NotAllowedActionDoesNotExist` | `Allow: Create\|Read` → calling `_u` → error "unknown operation" (the router's), not a stub |
| `TestTable_HookWithoutActionFailsMount` | `Update` hook with `Allow: Read` → `ErrHookWithoutAction` |
| `TestTable_CreateSetsIDAndStamps` | create without id → stored row has id, `created_at == updated_at > 0` |
| `TestTable_BatchOnNonTransactionalStoreRejected` | a Store without `Atomic`; create 2 records → `ErrBatchNotAtomic`; 1 record → stored |
| `TestTable_CreateBatchIsAtomic` | 2 records, hook fails on the 2nd → 0 rows stored |
| `TestTable_UpdateOnlyListedFields` | row {name:"a", note:"x"}; update fields `["name"]` with record {name:"b"} → stored {name:"b", note:"x"} |
| `TestTable_UpdateMissingIsNotFound` | update id "nope" → error kind `model.ErrorNotFound` |
| `TestTable_UpdateHookSeesOldAndNext` | hook rejects when `old.name == "locked"` → field error returned, row unchanged |
| `TestTable_CommittedAfterCommitOnly` | hook error → `Committed` never called; success → called once per record |
| `TestTable_CommittedUpdateCarriesEachID` | update ids `["a","b"]` → `Committed` sees id `a` then id `b`, with `fields` |
| `TestTable_SearchDefaultUsesItemLabel` | rows "Ana", "Bruno"; search "an" → only "Ana" |
| `TestTable_SearchHookOrIsRejected` | `Search` returns `storage.Or(storage.Eq(...))` → `ErrOrCondition` |
| `TestTable_FilterEquality` | filter {status:"open"} → only open rows |
| `TestTable_CompositeKeyRoundTrip` | table with PK (room_id, equipment_id); create `{room_id:"r1", equipment_id:"e7"}`, read `{id:"r1/e7"}` → the row; delete ids `["r1/e7"]` → gone |
| `TestTable_CompositeKeyPartRequired` | create without `equipment_id` → field error on `equipment_id` |
| `TestTable_KeyWithSeparatorRejected` | create `{room_id:"r/1", …}` → `ErrKeyHasSeparator`; a single-key table with id `"a/b"` → `ErrKeyHasSeparator` |
| `TestTable_CompositeKeyWithInt` | PK (shift_id text, specific_date int64); create; read `{id:"s1/1700000000"}` → the row |
| `TestTable_PublicCreateWithoutSession` | `Mount(reg, []Public{{&T{}, model.Create}}, m)`; route `_c` has public access; hook sees `UserID == ""`; `_r` still requires a session |
| `TestTable_ServerOnlyRejected` | create with a `ServerOnly` field set → `ErrServerOnlyField` (existing check, through Table) |

## Stage 6 — translations and documentation

`langc sync .` (zero pending, zero reuse warnings). `README.md`: replace the `Registry.Create/Update/Delete` sections with
«A table: declare `Allow` in its `model.Definition`; `ormc` generates `Mount<Entity>`; write rules only» and «A report: `Registry.Read`».
`docs/ARCHITECTURE.md`: the engine (create/update/delete/read flows above as a mermaid `flowchart TD`), rule R1 and why.

## Delivery check

```bash
gotest
langc sync .
grep -rn "webtyp.com/orm" .                                            # empty
grep -rn "func (r \*registry) \(Create\|Update\|Delete\)(" .          # empty
```
A PR that does not compile, or whose tests could not run, is a failed delivery.

| Stage | Files |
|---|---|
| 1 | `contract.go`, `store.go` |
| 2 | `table.go` |
| 3 | `table_create.go`, `table_update.go`, `table_delete.go`, `table_read.go` |
| 4 | `create.go`, `update.go`, `delete.go` |
| 5 | `storetest/storetest.go`, `tests/*.go`, `docs/TESTS.md` |
| 6 | `lang.json`, `README.md`, `docs/ARCHITECTURE.md` |
