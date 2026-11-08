---
PLAN: "feat!: closed contract — crud.Module, crud.Registry, crud.Route, crud.Caller, typed read filter, ServerOnly, crud.Loopback"
EXECUTOR: antigravity
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit after every stage and stop. Never ask whether to
> proceed. This plan runs **locally**: do not tag, push or publish anything; the owner reviews first.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.

# Plan: the closed CRUD contract

Context (master `webtyp/docs/ARCHITECTURE_MASTER_PLAN.md`, decisions D5–D14; diagram
`webtyp/docs/diagrams/TRANSPORT_CRUD_ARCHITECTURE.md`): today a module receives `router.OperationRegistry` and can
register any handler, read `args.TenantId` from the client (IDOR) or chain `.Public()`. After this plan a module
receives **only** `crud.Registry` (4 methods), gets a `crud.Route` with no `Public`/`Authenticated`/`Requires`, learns
who calls through `crud.Caller`, and every request goes through `validate.go`. Transports (rpc, mcp, dbsync, the
loopback) call `crud.Mount`.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```
Local `replace` in `go.mod` (do not remove them; publishing does):
```
replace webtyp.com/model => ../model
replace webtyp.com/router => ../router
```
Remove `webtyp.com/rpc` from `go.mod` (`go mod tidy`). Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`.

## APIs used

| Symbol | Signature | Where |
|---|---|---|
| `router.OperationRegistry` | `interface{ Operation(name string, h HandlerFunc) Route }` | router `router.go:192` |
| `router.Route` | `Requires(model.Resource, model.Action) Route; Authenticated() Route; Public() Route; RequiresMFA() Route; Accepts(model.Fielder) Route; Describe(string) Route` | router `route.go` |
| `router.Context.UserID` | `UserID() string` | router `router.go:16` |
| `router.WriteError` | `func WriteError(ctx Context, err error)` | router (local, router plan stage 1) |
| `router.Caller` | `Call(op string, args model.Encodable, into model.Decodable, done func(error)); Dispatch(op string, args model.Encodable)` | router `caller.go` |
| `router.Problem`, `router.KindOfStatus` | see router `problem.go` | |
| `model.Field.ServerOnly` | `bool` | model (local, A0) |
| `model.Model`, `model.ModelSlice`, `model.ModuleNaming`, `model.Fielder` | `webtyp/model/interface.go` | |
| `model.Resource(name)`, `model.Read/Create/Update/Delete` | | model `rbac.go` |
| `model.ReadStringPtr`, `model.IsZeroPtr`, `model.ActionCreate` | used today in `validate.go` | |
| `model.ValidateInput` | `func ValidateInput(action byte, f Fielder) error` — `ValidateFields` that skips `ServerOnly` fields | model (local, A0) |

If a signature differs, stop and name the symbol; never adapt by guessing.

## Stage 1 — `contract.go` (new): the types modules see

```go
package crud

import "webtyp.com/model"

// Caller is who runs the operation, filled by crud from the authenticated session —
// never from the request body. UserID is "" only on a read the app declared public.
//
// Example: func(c crud.Caller, list model.ModelSlice) error { return m.CreatePayments(c.UserID, …) }
type Caller struct {
	UserID string
}

// Route is what a module may still say about one of its operations. It has no Public,
// Authenticated or Requires: crud fixes the permission to Resource(ModelName) + the action,
// and only the app (appkit PublicReads) can open a read to visitors.
type Route interface {
	RequiresMFA() Route
	Describe(text string) Route
}

// Registry is the only thing a module receives in MountOperations: the four standard operations.
type Registry interface {
	Create(newList func() model.ModelSlice, create func(c Caller, list model.ModelSlice) error) Route
	Read(m model.ModuleNaming, newFilter func() model.Model, read func(c Caller, args ReadArgs, filter model.Model) (model.ModelSlice, error)) Route
	Update(newRecord func() model.Model, update func(c Caller, ids, fields []string, rec model.Model) error) Route
	Delete(m model.ModuleNaming, remove func(c Caller, ids []string) error) Route
}

// Module is a domain module: it names itself and mounts its entities' operations.
//
//	var _ crud.Module = (*Module)(nil)
type Module interface {
	model.ModuleNaming
	MountOperations(reg Registry)
}
```
`newFilter` may be nil (no filter): `read` then receives `filter == nil`.

## Stage 2 — `mount.go` (new) replaces `mountOp` and the four exported `Mount*` functions

Delete from `op.go`: `mountOp`. Delete the exported functions `MountCreate` (`create.go`), `MountRead` (`read.go`),
`MountUpdate` (`update.go`), `MountDelete` (`delete.go`); their handler bodies move into the registry methods below,
with `rpc.WriteError` → `router.WriteError` and the user's callback receiving `Caller`.
Grep: `grep -rn "func Mount\(Create\|Read\|Update\|Delete\)\|rpc\." --include=*.go .` → empty.

```go
package crud

import (
	"webtyp.com/model"
	"webtyp.com/router"
)

// mountError is a wiring mistake found while mounting, before any request is served.
type mountError string

func (e mountError) Error() string { return string(e) }

const (
	ErrNilFunction    mountError = "crud: Create, Read, Update or Delete got a nil function"
	ErrEmptyModelName mountError = "crud: entity with empty ModelName()"
)

// Mount mounts m's operations on a transport's registry: every operation gets its canonical
// name Op(entity, action), Requires(Resource(entity.ModelName()), action) and the validation of
// validate.go. A Read whose entity name is in public is mounted Public instead (only the app
// passes public). The first wiring mistake is returned and the transport must not serve.
//
//	if err := crud.Mount(reg, nil, patients); err != nil { return err }
func Mount(reg router.OperationRegistry, public []model.ModuleNaming, m Module) error {
	r := &registry{reg: reg, public: public}
	m.MountOperations(r)
	return r.err
}

type registry struct {
	reg    router.OperationRegistry
	public []model.ModuleNaming
	err    error
}

func (r *registry) fail(err error) Route {
	if r.err == nil {
		r.err = err
	}
	return noRoute{}
}

func (r *registry) isPublic(name string) bool {
	for _, p := range r.public {
		if p.ModelName() == name {
			return true
		}
	}
	return false
}

// route wraps router.Route so the module only sees RequiresMFA and Describe.
type route struct{ r router.Route }

func (x route) RequiresMFA() Route        { x.r.RequiresMFA(); return x }
func (x route) Describe(text string) Route { x.r.Describe(text); return x }

// noRoute is returned after a wiring mistake; Mount reports the error.
type noRoute struct{}

func (noRoute) RequiresMFA() Route      { return noRoute{} }
func (noRoute) Describe(string) Route   { return noRoute{} }

func callerOf(ctx router.Context) Caller { return Caller{UserID: ctx.UserID()} }
```

Each method (put Create in `create.go`, Read in `read.go`, Update in `update.go`, Delete in `delete.go`, as methods of `*registry`):

```go
func (r *registry) Create(newList func() model.ModelSlice, create func(c Caller, list model.ModelSlice) error) Route {
	if newList == nil || create == nil {
		return r.fail(ErrNilFunction)
	}
	sample := newList().Append().(model.Model)
	if sample.ModelName() == "" {
		return r.fail(ErrEmptyModelName)
	}
	schema := sample.Schema()
	h := func(ctx router.Context) {
		in := createIn{list: newList()}
		if err := ctx.Decode(&in); err != nil {
			router.WriteError(ctx, ErrCannotRead)
			return
		}
		if err := validateCreateBatch(schema, in.list, in.tooMany); err != nil {
			router.WriteError(ctx, err)
			return
		}
		if err := create(callerOf(ctx), in.list); err != nil {
			router.WriteError(ctx, err)
			return
		}
		last := in.list.At(in.list.Len() - 1).(model.Encodable)
		if err := ctx.Encode(last); err != nil {
			router.WriteError(ctx, err)
		}
	}
	rt := r.reg.Operation(Op(sample, model.Create), h).Requires(model.Resource(sample.ModelName()), model.Create)
	rt.Accepts(sample)
	return route{rt}
}
```
`Update` mirrors today's `MountUpdate` body the same way (nil check, `ModelName` check, `update(callerOf(ctx), in.IDs, in.Fields, in.Record)`).
`Delete` mirrors `MountDelete` (`remove(callerOf(ctx), in.IDs)`), `Accepts` nil.

`Read`:
```go
func (r *registry) Read(m model.ModuleNaming, newFilter func() model.Model, read func(c Caller, args ReadArgs, filter model.Model) (model.ModelSlice, error)) Route {
	if m == nil || read == nil {
		return r.fail(ErrNilFunction)
	}
	if m.ModelName() == "" {
		return r.fail(ErrEmptyModelName)
	}
	h := func(ctx router.Context) {
		args := ReadArgs{}
		if newFilter != nil {
			args.Filter = newFilter()
		}
		if err := ctx.Decode(&args); err != nil {
			router.WriteError(ctx, ErrCannotRead)
			return
		}
		if err := validateReadArgs(&args); err != nil {
			router.WriteError(ctx, err)
			return
		}
		list, err := read(callerOf(ctx), args, args.Filter)
		if err != nil {
			router.WriteError(ctx, err)
			return
		}
		if enc, ok := list.(model.Encodable); ok {
			if err := ctx.Encode(enc); err != nil {
				router.WriteError(ctx, err)
			}
		}
	}
	rt := r.reg.Operation(Op(m, model.Read), h)
	if r.isPublic(m.ModelName()) {
		rt.Public()
	} else {
		rt.Requires(model.Resource(m.ModelName()), model.Read)
	}
	rt.Accepts(&ReadArgs{})
	return route{rt}
}
```

## Stage 3 — `ReadArgs.Filter` and `validate.go`

`read.go`: add `Filter model.Model` to `ReadArgs` (last field; comment: "the entity's typed filter, nil when the
entity declares none; decoded from the key `filter`"). Add constant `keyFilter = "filter"` in `op.go`.
`EncodeFields`: after `limit`, `if a.Filter != nil { w.Object(keyFilter, a.Filter) }`.
`DecodeFields`: after `limit`, `if a.Filter != nil { r.Object(keyFilter, a.Filter) }`. Check `model.FieldWriter`/
`FieldReader` for the exact `Object` method name and signature before writing; `UpdateArgs` already uses them (`update.go`).
`ReadArgsModel` and `Schema()` do not describe the filter (MCP tools see id/search/after/limit only); say so in README.
Update the comment of `Cap`: remove the example where a module calls it — crud applies it.

`errors.go`: add `ErrArgTooLong model.Invalid = "crud: argument too long"` and
`ErrServerOnlyField model.Invalid = "crud: field written only by the server"`.

`validate.go`:
```go
// validateReadArgs rejects oversized arguments (never truncates them), caps the limit
// and validates the typed filter against its schema.
func validateReadArgs(a *ReadArgs) error {
	if len(a.ID) > MaxID {
		return ErrInvalidID
	}
	if len(a.Search) > MaxSearch || len(a.After) > MaxAfter {
		return ErrArgTooLong
	}
	a.Limit = Cap(a.Limit)
	if a.Filter == nil {
		return nil
	}
	return model.ValidateInput(model.ActionCreate, a.Filter)
}
```
`validateCreateBatch(schema []model.Field, list model.ModelSlice, tooMany bool)`: before the schema validation of each
record, reject any `ServerOnly` field that carries a value:
```go
func serverOnlyWritten(schema []model.Field, rec model.Fielder) bool {
	ptrs := rec.Pointers()
	for i := range schema {
		if !schema[i].ServerOnly {
			continue
		}
		ft := schema[i].Type.Storage()
		switch ft {
		case model.FieldText, model.FieldRaw:
			if v, _ := model.ReadStringPtr(ptrs[i]); v != "" {
				return true
			}
		default:
			if !model.IsZeroPtr(ptrs[i], ft) {
				return true
			}
		}
	}
	return false
}
```
→ `if serverOnlyWritten(schema, elem) { return ErrServerOnlyField }`, then replace the current
`model.Validator` / `model.ValidateFields` branch with `model.ValidateInput(model.ActionCreate, elem)`: a `NotNull` `ServerOnly`
field (e.g. `tenant_id`) arrives empty and must pass; the module fills it. Grep: `grep -n "ValidateFields\|model.Validator" validate.go` → empty.
`validateUpdateFields`: after finding `field := &schema[idx]`, `if field.ServerOnly { return ErrServerOnlyField }`.

## Stage 4 — `loopback.go` (new): `crud.Loopback`, port of `webtyp.com/router` v0.4.18 `loopback/loopback.go`

Copy that file into this package as `loopback.go` and change only this:
- Public API (delete `New`, `WithTenant`, `ActingAs` names and all tenant injection: `tenantID`, `acceptsHasTenant`, the
  `{"tenant_id":…}` body; nil args always send `{}`):
```go
// Loopback is an in-process router.Caller over real modules, for demos, offline apps and tests:
// same operation names as rpc and mcp ("<ModelName>.<Op(entity, action)>"), no transport and
// no RBAC. It panics on a wiring mistake (crud.Mount's error) — it only runs in demos and tests.
//
//	caller := crud.Loopback(patients)
func Loopback(mods ...Module) router.Caller { return newLoopback("", mods) }

// LoopbackActingAs is Loopback signed in as userID: operations see it in Caller.UserID.
//
//	caller := crud.LoopbackActingAs("u1", payments)
func LoopbackActingAs(userID string, mods ...Module) router.Caller { return newLoopback(userID, mods) }
```
- In `newLoopback`, for each non-nil module: empty `ModelName()` panics with the existing message (prefix `crud: loopback:`);
  set `reg.module = m.ModelName()`; `if err := Mount(reg, nil, m); err != nil { panic(err.Error()) }`.
- Rename the unexported types to avoid clashes with stage 2: `registry`→`loopReg`, `opEntry`→`loopOp`, `noopRoute`→`loopRoute`,
  `caller`→`loopCaller`, `inCtx`→`loopCtx`, `callError`→`loopError`. `loopRoute.Accepts` keeps no state (tenant injection is gone).
- `crud` already requires `webtyp.com/json` and `webtyp.com/fmt`.

## Stage 5 — tests (`tests/`, external package, `crud.Loopback` replaces `router/loopback` everywhere)

Rewrite `tests/mount_test.go`, `tests/security_test.go`, `tests/args_json_test.go`, `tests/bench_test.go` onto
`MountOperations(reg crud.Registry)`; every `loopback.New(mod)` → `crud.Loopback(mod)`. Rename the tests
`TestMountCreate_*`→`TestCreate_*`, `TestMountRead_*`→`TestRead_*`, `TestMountUpdate_*`→`TestUpdate_*`,
`TestMountDelete_*`→`TestDelete_*`. Change `TestSecurity_SearchTermCapped` and `TestSecurity_CursorAndIDCapped` to expect
rejection: Search of 257 bytes → error with `model.KindOf == model.ErrorInvalid` and text containing `argument too long`;
ID of 129 bytes → `crud: empty id` (ErrInvalidID). New tests (setup → call → expected):

| Test (file) | Setup | Call | Expected |
|---|---|---|---|
| `TestCaller_UserIDFromSession` (`tests/caller_test.go`) | module whose Create stores `c.UserID` | `crud.LoopbackActingAs("u1", mod).Call(<module>.test_record_c, {records:[{name:"a"}]})` | stored UserID `"u1"` |
| `TestCaller_AnonymousLoopback` | same | `crud.Loopback(mod)` | stored UserID `""` |
| `TestRead_FilterDecodedAndValidated` (`tests/filter_test.go`) | entity with filter type `byName{Name string}` (schema `name` Text NotNull) | Call `_r` with `{"filter":{"name":"ana"}}` | read receives `filter.(*byName).Name == "ana"` |
| `TestRead_FilterInvalidIs400` | same | `{"filter":{"name":""}}` | ErrorInvalid, field `name`; read not called |
| `TestRead_NilFilter` | `newFilter` nil | `{}` | read receives `filter == nil` |
| `TestServerOnly_CreateRejected` (`tests/server_only_test.go`) | schema field `owner` with `ServerOnly: true` | create `{records:[{name:"a",owner:"x"}]}` | error text `crud: field written only by the server`; create not called |
| `TestServerOnly_CreateEmptyAllowed` | same, and `owner` also `NotNull` | `{records:[{name:"a"}]}` | create called (NotNull ServerOnly is not required from the client) |
| `TestServerOnly_UpdateRejected` | same | update fields `["owner"]` | `crud: field written only by the server` |
| `TestMount_PublicRead` (`tests/public_test.go`) | collecting `router.OperationRegistry` fake recording Public/Requires per name | `crud.Mount(fake, []model.ModuleNaming{&mockRecord{}}, mod)` | `test_record_r` is Public with no Requires; `test_record_c` Requires(`test_record`, Create) |
| `TestMount_NotPublicByDefault` | same fake | `crud.Mount(fake, nil, mod)` | `test_record_r` Requires(`test_record`, Read), not Public |
| `TestMount_NilFunctionIsError` | module calling `reg.Read(&mockRecord{}, nil, nil)` | `crud.Mount(fake, nil, mod)` | `== crud.ErrNilFunction` |
| `TestMount_RouteHasNoPublic` (compile-level) | `var _ interface{ RequiresMFA() crud.Route; Describe(string) crud.Route } = crud.Route(nil)` and a comment that `Public` does not exist | — | compiles |

Add each test to `docs/TESTS.md` (create if missing: table `Test | Rule`).

## Stage 6 — documentation

`README.md` (≤150 lines; `Why This Exists` with a Without/With: `reg.Operation` + `args.TenantId` vs `MountOperations(reg crud.Registry)`;
`Before You Start`; `Step-by-Step Guide` with a complete compilable module: entity, `MountOperations` using the four methods,
a typed filter, `crud.Caller`, `ServerOnly`, a test with `crud.Loopback`; `Documentation Map`). `docs/ARCHITECTURE.md`:
the flow transport → `crud.Mount` → registry → `validate.go` → module, public reads, why no `Public()` on `Route`.
Remove from `docs/*` every mention of `MountCreate/MountRead/MountUpdate/MountDelete` and `rpc.WriteError`.

## Delivery check

`gotest` passes; `grep -rn "webtyp.com/rpc\|router/loopback\|func Mount\(Create\|Read\|Update\|Delete\)" --include=*.go .` → empty;
`grep -n "webtyp.com/rpc" go.mod` → empty. A tree that does not compile or whose tests did not run is a failed delivery.

| Stage | Files |
|---|---|
| 1 | `contract.go` |
| 2 | `mount.go`, `op.go`, `create.go`, `read.go`, `update.go`, `delete.go` |
| 3 | `read.go`, `op.go`, `errors.go`, `validate.go` |
| 4 | `loopback.go` |
| 5 | `tests/*.go`, `docs/TESTS.md` |
| 6 | `README.md`, `docs/ARCHITECTURE.md` |
