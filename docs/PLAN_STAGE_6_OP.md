← [Stage 5](PLAN_STAGE_5_DOCS.md) | Next → end

# Stage 6 — `router.Operation` replaces `crud.Op` (one way to name an operation)

Do this stage **before** stage 5's documentation commit, so the docs describe the final API; the stage numbers only order the files. `crud` ships **one** breaking version with every change of stages 1–6.

Prerequisite: `router` v0.4.20 is published (`go get webtyp.com/router@v0.4.20`, done in the plan's Tools).
The `router` plan (`router/docs/PLAN.md`) must be executed first: it defines `router.Operation` and changes `router.OperationRegistry` and `router.Caller`.

## 6.1 What `crud` registers

`crud.Op(m, action)` and its string names (`<entity>_c|_r|_u|_d`) disappear. The module is part of the identity now, so transports stop tracking it out of band.

| File | Old | New |
|---|---|---|
| `op.go` | `func Operation(m model.ModuleNaming, action model.Action) string` and its comment | **deleted** (keep `keyRecords`, `keyIDs`, `keyFields`, `keyRecord`, `keyFilter`, `statusNoContent` and `decodeTextList`) |
| `stored_entity.go` `mountAction` | `op := r.reg.Operation(Operation(entity.Model, action), h)` | `rt := r.reg.Operation(router.Operation{Module: r.module, Entity: modelName, Action: action}, h)`; rename the variable `op` → `rt` in the rest of the closure (`rt.Public()`, `rt.Requires(…)`, `rt.Accepts(entity.Model)`, `rt.RequiresMFA()`, `rt.Describe(…)`) |
| `read.go` `Registry.AddReadOnlyEntity` | `rt := r.reg.Operation(Operation(m, model.Read), h)` | `rt := r.reg.Operation(router.Operation{Module: r.module, Entity: m.ModelName(), Action: model.Read}, h)` |
| `create.go:9`, `update.go:10`, `delete.go:10` (doc examples) | `caller.Call(crud.Op(&Device{}, model.Create), …)` | `caller.Call(router.Operation{Module: "device_manager", Entity: "device", Action: model.Create}, …)` (same for Update and Delete) |

## 6.2 `loopback.go` — keyed by `router.Operation`

Replace `loopbackRegistry`, `loopbackOperation`, `Operation`, `find`, `Call` (head) and `Dispatch`; the rest of the file does not change. `loopbackRegistry.module` disappears (the module is inside the `Operation`), and so does the line in `newLoopbackCaller` that sets it (`reg.module = name`);
keep the check that a module's `ModelName()` is not empty.

```go
type loopbackRegistry struct {
	ops []loopbackOperation
}

type loopbackOperation struct {
	op router.Operation
	h  router.HandlerFunc
}

func (r *loopbackRegistry) Operation(op router.Operation, h router.HandlerFunc) router.Route {
	r.ops = append(r.ops, loopbackOperation{op: op, h: h})
	return loopbackRoute{}
}

func (r *loopbackRegistry) find(op router.Operation) (loopbackOperation, bool) {
	for i := range r.ops {
		if r.ops[i].op == op {
			return r.ops[i], true
		}
	}
	return loopbackOperation{}, false
}

func (c *loopbackCaller) Call(op router.Operation, args model.Encodable, into model.Decodable, done func(err error)) {
	entry, ok := c.reg.find(op)
	if !ok {
		if done != nil {
			done(fmt.Err("loopback", "unknown", "operation", op.String()))
		}
		return
	}
	// …the rest of Call is unchanged…
}

func (c *loopbackCaller) Dispatch(op router.Operation, args model.Encodable) {
	c.Call(op, args, nil, nil)
}
```

Update the doc comment of `Loopback`: operations are addressed by `router.Operation`, "no transport and no RBAC".

## 6.3 Tests

In `tests/helpers_test.go` add

```go
// operationOf is the operation of the test module "tmod" and its entity "t" for one action.
func operationOf(a model.Action) router.Operation { return router.Operation{Module: "tmod", Entity: "t", Action: a} }
```

and replace every `"tmod."+crud.Op(&tRec{}, model.X)` by `operationOf(model.X)`; in `filter_test.go` the module is `filtertest` and the entity the wire model's name: build the `router.Operation` literal there (`router.Operation{Module: "filtertest", Entity: "mock", Action: model.Read}`).
`mockRouter.Operation` in `tests/stored_entity_test.go` takes a `router.Operation` and records `op.Entity + "_" + op.Action.Verb()` (`Verb` lives on `model.Action`, not on `router.Operation`); replace the assertions `reg.Has("t_u")` by `reg.Has("t_update")` and `reg.Has("t_c")` by `reg.Has("t_create")`.

New tests in `tests/op_test.go`:

| Test | setup → call → expected |
|---|---|
| `TestOperation_StoredEntityMountsOneOperationPerAllowedAction` | `Allow: Create\|Read`; `Mount` with a `mockRouter` → `Has("t_create")`, `Has("t_read")`, not `Has("t_update")`, not `Has("t_delete")` |
| `TestOperation_WireReadMountsItsOwnOperation` | the `filtertest` module → the registry received exactly `router.Operation{Module:"filtertest", Entity:"mock", Action: model.Read}` |
| `TestOperation_ModuleIsPartOfTheIdentity` | two modules each with entity `t` mounted on the same loopback → calling `router.Operation{Module:"a", Entity:"t", Action: model.Read}` reaches only module `a`'s table |
| `TestOperation_UnknownOperationIsAnError` | loopback call with `router.Operation{Module:"nope", Entity:"t", Action: model.Read}` → `done` receives an error containing `nope.t.read` |

## 6.4 `ReadArgs` writes only what it was given

A read now travels in a URL (`GET …?search=12.345`), so `ReadArgs.EncodeFields` (`read.go:60`) must not write empty values. Replace its body:

```go
func (a *ReadArgs) EncodeFields(w model.FieldWriter) {
	if a.ID != "" {
		w.String("id", a.ID)
	}
	if a.Search != "" {
		w.String("search", a.Search)
	}
	if a.After != "" {
		w.String("after", a.After)
	}
	if a.Limit != 0 {
		w.Int("limit", a.Limit)
	}
	if a.Filter != nil {
		w.Object(keyFilter, a.Filter)
	}
}
```

Decoding already treats a missing key as the zero value, so nothing else changes. Test (`tests/read_test.go`): `TestReadArgs_EncodesOnlySetFields` — `ReadArgs{}` encoded with a recording writer
writes no field at all; `ReadArgs{Search:"x"}` writes only `search`; the existing `TestReadArgs_RoundTrip` stays green.

## 6.5 Delivery additions

`grep -rn "crud\.Operation(\|func Operation(" --include=*.go .` → empty. `StoredEntity`/`Registry` documentation says operations are addressed by `router.Operation`.
Commit: `refactor!: router.Operation replaces crud.Op`.
