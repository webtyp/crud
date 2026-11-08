# crud
<img src="docs/img/badges.svg">

Standard create, read, update, and delete operation contract between module backends and consumers: `crud.Module`, `crud.Registry`, `crud.Route`, `crud.Caller`, typed read filters, and `crud.Loopback`.

## Why This Exists

`crud` enforces a closed, declarative boundary for CRUD operations across the ecosystem:

| Without `crud.Registry` | With `crud.Registry` |
|---|---|
| A module received `router.OperationRegistry` and could mount arbitrary routes, read raw context, or expose internal parameters | A module receives `crud.Registry` to mount Tables and Reads |
| Modules read `args.TenantId` from untrusted client bodies (IDOR vulnerability) | The server provides `crud.Caller` with `UserID` filled from verified session context |
| Modules manually chained `.Public()` on routes, leaking sensitive operations | Only the application root declares public reads via `App.PublicReads` passed to `crud.Mount` |
| Input validation and server-only field protections were duplicated across modules | `validate.go` centralized defense rejects oversized inputs, enforces schema, and protects `ServerOnly` fields |

## Before You Start

- Models implement `model.Model` / `model.ModelSlice`.
- Entities register via `crud.Module` and receive `crud.Registry`.
- `ReadArgsModel` and its `Schema()` describe wire arguments for MCP tools (`id`, `search`, `after`, `limit`). Typed filters travel under key `"filter"`.
- `crud.Cap(limit)` is automatically applied to `ReadArgs.Limit` (bounded to `1..MaxPage`).

## Step-by-Step Guide

A table: declare `Allow` in its `model.Definition`; `ormc` generates `Mount<Entity>`; write rules only.

A report: `Registry.Read`.

Testing with `crud.Loopback`:

```go
func TestDeviceOps(t *testing.T) {
	mod := &Module{}
	caller := crud.LoopbackActingAs("u1", mod)
	op := "device." + crud.Op(&Device{}, model.Create)

	var saved Device
	caller.Call(op, &crud.CreateArgs{Records: []model.Model{&Device{Name: "Switch"}}}, &saved, func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	})
}
```

## Documentation Map

| Document | Purpose |
|---|---|
| [`contract.go`](contract.go) | Public interfaces: `Module`, `Registry`, `Route`, `Caller`, `Table`, `Store`, `Tx` |
| [`table.go`](table.go) | Engine mounting `crud.Table` operations for allowed actions |
| [`store.go`](store.go) | `Store`, `Tx` persistence interfaces and transaction capabilities |
| [`mount.go`](mount.go) | `crud.Mount` implementation for transports (rpc, mcp, dbsync) |
| [`read.go`](read.go) | `ReadArgs`, `Cap`, `MaxPage`, and `Registry.Read` for report/wire models |
| [`validate.go`](validate.go) | Centralized input and security boundary validation |
| [`loopback.go`](loopback.go) | In-process `crud.Loopback` and `crud.LoopbackActingAs` |
| [`storetest/`](storetest/storetest.go) | Conformance test suite for `crud.Store` implementations |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Architectural layout and request flow |
| [`docs/TESTS.md`](docs/TESTS.md) | Test suite map |
