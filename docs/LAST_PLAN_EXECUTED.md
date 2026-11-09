---
PLAN: "feat!: the write flow is tested at its edges, a simple write has no transaction, a duplicate key is ErrAlreadyExists and Revision rejects a stale update; storetest becomes conformance"
EXECUTOR: antigravity
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit after every stage and stop. Never ask whether to
> proceed. This plan runs **locally**: do not tag, push or publish anything; the owner reviews first.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.

Stages: 1 edge-case tests · 2 `storetest` → `conformance` · 3 no transaction for a simple write · 4 docs and measurements · 5 duplicate key and `Revision` (D48).

## Goal

`docs/diagrams/WRITE_FLOW.md` promises branches that no test walks (measured on `crud` v0.0.7: 73% of statements; `stored_entity_create.go` 83%, `stored_entity_update.go` 73%, `stored_entity_delete.go` 68%).
Stage 1 adds the missing tests; they pin down what the code already does, and if one fails the code has a bug: report it in the PR, never weaken the expectation.
Stage 2 renames the conformance suite `storetest` → `conformance`, the name `files` and `router` already use for «the suite that proves an implementation of this contract».
Stage 3 applies decision D48 where it needs no new contract: a write of **one record with no business rule** runs without a transaction (measured: on Postgres the transaction adds 14% to a single insert, `orm/docs/PERFORMANCE.md`).
Stage 5 applies the rest of D48 now that `storage` v0.2.0 reports duplicates and rows affected: the duplicate-key error and the `Revision` conditional update.

## Ecosystem search

Searched `storetest`, `conformance` over `webtyp/*`: `storetest` is imported only by `orm/tests/store_test.go` (the `orm` plan renames its import); `router/conformance` and `files/conformance` are the same idea under the name to use (**reuse the name**). No new library.

## Design gate

1. **Prior art**: `files/conformance` (`conformance.Run(t, Factory{…})`), `router/conformance`.
2. **Novice-name test**: `conformance.Run(t, newStore)` = «run the conformance suite against my store».
3. **Ledger**: concepts 0; names for «the suite that proves an implementation» 2 (`storetest`, `conformance`) → 1.
4. **Where**: the suite stays in `crud` (the repo that owns the `Store` contract); see `docs/diagrams/PLACEMENT.md`.
5. **Deletes**: the directory `storetest/`.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go get webtyp.com/storage@v0.2.0
go get webtyp.com/model@v0.2.18
go get webtyp.com/definitioncheck@v0.0.3
go mod tidy
```

## Stage 1 — tests for the uncovered branches (`tests/write_edges_test.go`, package `tests`)

Helpers (same file). A store that fails on demand, and a request body the decoder refuses:

```go
var errStoreDown = fmt.Err("store down")

// failingStore is the memory store whose transaction fails on the chosen write.
type failingStore struct {
	*memStore
	failOn string // "create", "update" or "delete"
}

func (s *failingStore) Atomic(fn func(crud.Tx) error) error {
	return s.memStore.Atomic(func(tx crud.Tx) error { return fn(&failingTx{Tx: tx, failOn: s.failOn}) })
}

type failingTx struct {
	crud.Tx
	failOn string
}

func (t *failingTx) Create(rec model.Model) error {
	if t.failOn == "create" {
		return errStoreDown
	}
	return t.Tx.Create(rec)
}
func (t *failingTx) UpdateFields(rec model.Model, id string, fields []string) error {
	if t.failOn == "update" {
		return errStoreDown
	}
	return t.Tx.UpdateFields(rec, id, fields)
}
func (t *failingTx) Delete(rec model.Model, id string) error {
	if t.failOn == "delete" {
		return errStoreDown
	}
	return t.Tx.Delete(rec, id)
}

// unreadableArgs encodes the given key with a value of the wrong shape, so the handler's decoder refuses the body.
type unreadableArgs struct{ key string }

func (a unreadableArgs) IsNil() bool                      { return false }
func (a unreadableArgs) EncodeFields(w model.FieldWriter) { w.String(a.key, "not what the decoder expects") }
```

(`fmt` is `webtyp.com/fmt`. If `unreadableArgs` is accepted by a decoder, read that operation's `DecodeFields` and choose the input it refuses; say so in the PR. Never weaken the expected error.)

The error text a caller sees through the loopback is `"loopback: 400 " + string(<crud error>)`, as in `TestStoredEntity_BatchOnNonTransactionalStoreRejected`. Every test builds its module like `TestStoredEntity_CreateSetsIDAndStamps`
(`testModule{table: crud.StoredEntity{Model: &tRec{}, NewList: …, Store: …, Rules: declareRules(model.AllActions, …)}}`) and calls through `crud.LoopbackActingAs(nil, "u1", mod)`, or `crud.LoopbackActingAs(rec, "u1", mod)` with an `eventRecorder` when events are asserted.

| Test | setup → call → expected |
|---|---|
| `TestWrite_UnreadableBodyIsRefused` | for create (`records`), update (`ids`) and delete (`ids`): call with `unreadableArgs{key}` → `loopback: 400 crud: cannot read the request` (`ErrCannotRead`); the store has no rows |
| `TestWrite_CreateTwiceWithTheSameIDDoesNotDuplicate` | no rule; create `tRec{ID: "fixed", Name: "a"}`, then create `tRec{ID: "fixed", Name: "b"}` with an `eventRecorder` → second call has no error and answers the STORED record (`Name == "a"`); the store holds 1 row; exactly 1 event was published (the skipped record publishes nothing) |
| `TestWrite_CompositeKeyEmptyIsRequired` | the composite-key model of `TestStoredEntity_CompositeKeyPartRequired`; create with ALL key parts empty → error whose text contains `required` and the first key field's name. (If that existing test already passes a partial key, keep it and add this one: the branch `len(keyFields) > 1` with an empty key is the one with no coverage) |
| `TestWrite_StoreFailureRollsBackAndPublishesNothing` | `failingStore{failOn: "create"}`, rule `Create` returning `ApplyAndPublish`, batch of 2 records → error contains `store down`; 0 rows; 0 events. Same for `failOn: "update"` (seed one row first; update it) and `failOn: "delete"`: the row is still there and 0 events |
| `TestWrite_UpdateIDsAreValidated` | `IDs: nil` → `ErrNoIDs`; `IDs: {""}` → `ErrInvalidID`; `crud.MaxPage+1` ids → `ErrBatchTooLarge` |
| `TestWrite_DeleteIDsAreValidated` | same three rows for delete |
| `TestWrite_UpdateFieldsAreValidated` | `Fields: nil` → `ErrNoFields`; `{"id"}` → `ErrInvalidField`; `{"name","name"}` → `ErrInvalidField`; `{"nope"}` → `ErrUnknownField` |
| `TestWrite_BatchOnNonTransactionalStoreRejectedForUpdateAndDelete` | the `nonTxStore` wrapper of `TestStoredEntity_BatchOnNonTransactionalStoreRejected`; update with 2 ids and delete with 2 ids → `ErrBatchNotAtomic`; with 1 id → no error |
| `TestWrite_IDWithSeparatorIsRejectedOnUpdateAndDelete` | single-column key; update and delete with id `"a/b"` → `ErrKeyHasSeparator` |
| `TestWrite_ZeroAnswerIsRefusedOnUpdateAndDelete` | `Update` and `Delete` rules returning `(0, nil)` → `ErrNoAnswer` (`crud: the rule answered nothing…`); the row is unchanged / still there; 0 events |
| `TestWrite_DeleteMissingIsNotFound` | delete an id that does not exist → an error of kind `model.ErrorNotFound` (as `TestStoredEntity_UpdateMissingIsNotFound`) |

Add each test to `docs/TESTS.md` (format: `| [TestName](../tests/write_edges_test.go) | Rule |`, Rule = the name without `Test`, underscore as one space).
Run `gotest`: the new tests must pass against the existing code (they describe what is promised). Commit: `test: the edge cases of the write flow`.
The branch «encoding the answer fails» (the last `ctx.Encode` of each handler) cannot be reached through the loopback; leave it uncovered and say so in `docs/TESTS.md`'s header note.

## Stage 2 — `storetest` becomes `conformance`

1. `git mv storetest conformance`; in `conformance/storetest.go` set `package conformance` and `git mv conformance/storetest.go conformance/conformance.go`.
2. `git mv tests/storetest_test.go tests/conformance_test.go`; its import becomes `webtyp.com/crud/conformance` and the call `conformance.Run(t, …)`.
3. `docs/TESTS.md`, `README.md` and `docs/ARCHITECTURE.md`: every `storetest` becomes `conformance` (one sentence in the README: «a `Store` implementation proves itself with `conformance.Run`»).
4. `grep -rn "storetest" --include=*.go --include=*.md .` → empty.

Commit: `refactor!: storetest becomes conformance`. The consumer (`orm`) adapts in its own plan.

## Stage 3 — a one-record write with no rule runs without a transaction

`inTransaction` (in `stored_entity.go`) stays for everything else. Add next to it:

```go
// runWrite runs fn with the store's own Tx when the write is simple (one record, no business rule: a single statement is atomic by itself) and inside a
// transaction otherwise. A rule always gets a transaction: its check and its write must be one unit.
func runWrite(s Store, simple bool, fn func(tx Tx) error) error {
	if simple {
		return fn(s)
	}
	return inTransaction(s, fn)
}
```

In the three handlers replace `inTransaction(entity.Store, func(tx Tx) error {` by `runWrite(entity.Store, simple, func(tx Tx) error {`, with `simple` computed once per request, right before the call:

| Handler | `simple :=` |
|---|---|
| `createRecordsHandler` | `request.list.Len() == 1 && entity.Create == nil` |
| `updateRecordsHandler` | `len(request.IDs) == 1 && entity.Update == nil` |
| `deleteRecordsHandler` | `len(request.IDs) == 1 && entity.Delete == nil` |

Nothing else in the handlers changes: the events are still published after the write succeeds, a failed single statement leaves nothing partial, and the batch check (`ErrBatchNotAtomic`) keeps applying to more than one record.
Write the tests first (red), in `tests/write_edges_test.go`, with a store that counts how many transactions it opened:

```go
// countingStore counts the transactions opened through Atomic.
type countingStore struct {
	*memStore
	atomics int
}

func (s *countingStore) Atomic(fn func(crud.Tx) error) error {
	s.atomics++
	return s.memStore.Atomic(fn)
}
```

| Test | setup → call → expected |
|---|---|
| `TestWrite_OneRecordWithoutRuleOpensNoTransaction` | `declareRules(AllActions, 0)`, `countingStore`; create 1 record, update it, delete it → `atomics == 0`; the rows changed as asked; with an `eventRecorder`, 3 events were published |
| `TestWrite_OneRecordWithRuleOpensOneTransaction` | rules declared with `AddMyBusinessRule` and hooks returning `ApplyAndPublish`; the same three calls → `atomics == 3` |
| `TestWrite_BatchOpensOneTransaction` | no rule; create a batch of 3 → `atomics == 1` |
| `TestWrite_SimpleWriteFailureLeavesNothing` | `failingStore{failOn: "create"}` over a non-`Transactional` wrapper, create 1 record → the error is returned, 0 rows, 0 events |

Run `gotest`. Commit: `refactor: a one-record write without a rule runs without a transaction`.

## Stage 4 — documentation and measurements

- `stored_entity.go` / `store.go`: the comment of `Tx` says «reads and writes one table's rows»; it reads and writes the rows of **the store's database**, so a rule may read and write other tables of the same tenant class through `tx`. Say exactly that, with a three-line example of a rule that updates a second table.
- `docs/ARCHITECTURE.md` section on the write flow and `docs/diagrams/WRITE_FLOW.md`: add the branch «one record and no rule → no transaction» before step 4, and in the walkthrough say a rule always has one.
- Re-run `go test ./tests -run '^$' -bench . -benchmem -count=6` and save it as `docs/bench/<new version>.txt`; update the `Engine_*` tables of `docs/PERFORMANCE.md` and say what moved (the in-memory store's transaction copies the store, so the simple path shows a large gain there that a real database does not have: say so).

## Stage 5 — duplicate key and `Revision` (decision D48)

Contracts first. `errors.go`:

```go
	ErrAlreadyExists model.Conflict = "crud: the key already exists"
	ErrConflict      model.Conflict = "crud: the record changed since you read it; read it again"
	ErrRevisionRequired model.Invalid = "crud: send the revision you read"
```
and next to `ErrBatchNotAtomic`'s mount errors: `const ErrNoConditionalUpdate mountError = "crud: StoredEntity declares a Revision field but its Store cannot update conditionally; use orm.TenantStore or implement crud.ConditionalUpdater"`.
`store.go`, after `Transactional`:

```go
// ConditionalUpdater is an optional Store capability: UpdateFieldsIf writes fields of the row id only if its column field still holds expected, and
// says how many rows it changed (0 = someone changed the row first). It is what makes a Revision check atomic; a Store without it cannot serve an entity that declares one.
type ConditionalUpdater interface {
	UpdateFieldsIf(rec model.Model, id string, fields []string, field string, expected int64) (rows int64, err error)
}
```

Behaviour (`stored_entity.go` + the three handlers; `revisionField := revisionFieldIndex(schemaFields)` is the index of the schema field with `IsRevision()`, or −1, computed once at mount next to `keyFields`):

| Where | Rule |
|---|---|
| mount (`AddStoredEntity`) | `revisionField >= 0` and `entity.Store` is not a `ConditionalUpdater` → `ErrNoConditionalUpdate` |
| create | with a revision field, `setIntField(rec, revisionField, 1)` before the rule runs; any `tx.Create` error for which `storage.IsDuplicate(err)` is true is returned as `ErrAlreadyExists` (every entity, with or without a revision) |
| update | the revision field's name in `fields` → `ErrInvalidField` (a client never writes it: add the check to `validateUpdateFields` through its `schema` argument). With a revision field: `expected := the revision value of request.Record`; `expected <= 0` → `ErrRevisionRequired`; the handler reads the stored row first (`tx.ReadOne(old, id)`; absent → not found); `next` gets `revision = expected+1`; the revision field name is appended to `fields` and the store call is `rows, err := store.(ConditionalUpdater).UpdateFieldsIf(next, id, fields, revisionName, expected)`; `rows == 0` → `ErrConflict`. A duplicate-key error is `ErrAlreadyExists` here too |
| update without a revision field | unchanged (`UpdateFields`) |
| delete | unchanged (a conditional delete is not part of D48) |

The business rule still runs before the store call and still sees `old` and `next`; the event carries the new revision.
Tests first (red), `tests/revision_test.go` (package `tests`). Fixture: `rRec` = the `tRec` of `tests/stored_entity_test.go` plus a field `Revision int64` (`{Name: "revision", Type: model.Int(), DB: &model.FieldDB{Revision: true}}`, pointer in `Pointers()`), `rRecList`, and the memory store gains the method `UpdateFieldsIf` (it applies the update only if the stored row's `field` equals `expected`, and returns 1 or 0):

| Test | setup → call → expected |
|---|---|
| `TestRevision_CreateStartsAtOne` | create an `rRec` → the stored row has `Revision == 1` |
| `TestRevision_UpdateBumpsTheRevision` | create, then update `name` with `Record.Revision = 1` → stored `Revision == 2`; the event's record has `Revision == 2` |
| `TestRevision_StaleUpdateIsAConflict` | create; update with `Record.Revision = 1` (ok, now 2); update again with `Record.Revision = 1` → `loopback: 409 crud: the record changed since you read it; read it again`; the row still has the first update's value |
| `TestRevision_UpdateWithoutRevisionIsRefused` | update with `Record.Revision = 0` → `ErrRevisionRequired` |
| `TestRevision_ClientCannotWriteTheRevision` | `Fields: {"revision"}` → `ErrInvalidField` |
| `TestRevision_StoreWithoutConditionalUpdateFailsMount` | an `rRec` entity over a store that is a plain `crud.Store` (no `UpdateFieldsIf`) → `Mount` returns an error containing `ConditionalUpdater` |
| `TestWrite_DuplicateKeyIsAlreadyExists` | a store whose `Create` returns `storage.Duplicate(errStoreDown)`; create → `loopback: 409 crud: the key already exists` (same for an update whose `UpdateFields` returns it) |

Add the rows to `docs/TESTS.md`; update `docs/ARCHITECTURE.md`, `docs/diagrams/WRITE_FLOW.md` (the `Revision` branch of update) and `docs/diagrams/PLACEMENT.md` if imports change. Commit: `feat!: duplicate key and Revision`.

## Delivery check

1. `gotest` passes, with `-race` green.
2. `langc sync .` → zero untranslated keys, zero reuse warnings (the new error texts of stage 5 and the renamed mount errors).
3. Coverage: `go test ./tests -coverpkg=webtyp.com/crud -coverprofile=c.out && go tool cover -func=c.out` shows `stored_entity_create.go`, `stored_entity_update.go` and `stored_entity_delete.go` handlers at **≥ 90%** (the encode branch aside); paste the three numbers in the PR.
4. `git diff --stat main` lists only: `tests/write_edges_test.go`, `tests/conformance_test.go` (moved), `conformance/conformance.go` (moved), `stored_entity.go`, `stored_entity_create.go`, `stored_entity_update.go`, `stored_entity_delete.go`, `store.go`, `errors.go`, `validate.go`, `lang.json`, `tests/revision_test.go`, `docs/TESTS.md`, `docs/PERFORMANCE.md`, `docs/bench/*`, `README.md`, `docs/ARCHITECTURE.md`, `docs/diagrams/WRITE_FLOW.md`.
