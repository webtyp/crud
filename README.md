# crud
<img src="docs/img/badges.svg">

Standard create, read, update, and delete operation contract between module backends and consumers: `crud.Module`, `crud.Registry`, `crud.Route`, `crud.Caller`, typed read filters, and `crud.Loopback`.

## Why This Exists

`crud` enforces a closed, declarative boundary for CRUD operations across the ecosystem:

| Without `crud.Registry` | With `crud.Registry` |
|---|---|
| A module received `router.OperationRegistry` and could mount arbitrary routes, read raw context, or expose internal parameters | A module receives `crud.Registry` with only 4 operations: `Create`, `Read`, `Update`, `Delete` |
| Modules read `args.TenantId` from untrusted client bodies (IDOR vulnerability) | The server provides `crud.Caller` with `UserID` filled from verified session context |
| Modules manually chained `.Public()` on routes, leaking sensitive operations | Only the application root declares public reads via `App.PublicReads` passed to `crud.Mount` |
| Input validation and server-only field protections were duplicated across modules | `validate.go` centralized defense rejects oversized inputs, enforces schema, and protects `ServerOnly` fields |

## Before You Start

- Models implement `model.Model` / `model.ModelSlice`.
- Entities register via `crud.Module` and receive `crud.Registry`.
- `ReadArgsModel` and its `Schema()` describe wire arguments for MCP tools (`id`, `search`, `after`, `limit`). Typed filters travel under key `"filter"`.
- `crud.Cap(limit)` is automatically applied to `ReadArgs.Limit` (bounded to `1..MaxPage`).

## Step-by-Step Guide

```go
package device

import (
	"webtyp.com/crud"
	"webtyp.com/model"
)

type Module struct{}

func (m *Module) ModelName() string { return "device" }

func (m *Module) MountOperations(reg crud.Registry) {
	reg.Create(func() model.ModelSlice { return &DeviceList{} }, func(c crud.Caller, list model.ModelSlice) error {
		return m.CreateDevices(c.UserID, *list.(*DeviceList))
	})
	reg.Read(&Device{}, func() model.Model { return &DeviceFilter{} }, func(c crud.Caller, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		f, _ := filter.(*DeviceFilter)
		return m.ReadDevices(c.UserID, args, f)
	})
	reg.Update(func() model.Model { return &Device{} }, func(c crud.Caller, ids, fields []string, rec model.Model) error {
		return m.UpdateDevices(c.UserID, ids, fields, *rec.(*Device))
	})
	reg.Delete(&Device{}, func(c crud.Caller, ids []string) error {
		return m.DeleteDevices(c.UserID, ids)
	})
}
```

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
| [`contract.go`](contract.go) | Public interfaces: `Module`, `Registry`, `Route`, `Caller` |
| [`mount.go`](mount.go) | `crud.Mount` implementation for transports (rpc, mcp, dbsync) |
| [`create.go`](create.go) | `CreateArgs` and `Registry.Create` |
| [`read.go`](read.go) | `ReadArgs`, `Cap`, `MaxPage`, and `Registry.Read` |
| [`update.go`](update.go) | `UpdateArgs` and `Registry.Update` |
| [`delete.go`](delete.go) | `DeleteArgs` and `Registry.Delete` |
| [`validate.go`](validate.go) | Centralized input and security boundary validation |
| [`loopback.go`](loopback.go) | In-process `crud.Loopback` and `crud.LoopbackActingAs` |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Architectural layout and request flow |
| [`docs/TESTS.md`](docs/TESTS.md) | Test suite map |
