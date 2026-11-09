← [Stage 3](PLAN_STAGE_3_HANDLERS.md) | Next → [Stage 5](PLAN_STAGE_5_DOCS.md)

# Stage 4 — `Mount` checks the module's Definition against what it mounted

## 4.1 `contract.go` — `Module` (replace the interface)

```go
// Module is a domain module: it names itself, declares its entities and mounts their operations.
// definitionc generates Definition and MountOperations from the module's definition.go; nobody writes them by hand.
//
//	var _ crud.Module = (*Module)(nil)
type Module interface {
	model.ModuleNaming
	// Definition is the module's declaration (its definition.go). Mount compares it with what MountOperations mounted.
	Definition() model.ModuleDefinition
	MountOperations(reg Registry)
}
```

## 4.2 `mount.go` — replace `Mount`, the `registry` struct (already extended in stage 3) and add `verify`

Keep `mountError`, `ErrNilFunction`, `ErrEmptyModelName`, `failMount`, `isPublicAction`, `mountedRoute`, `failedRoute` and `currentUserOf` as they are.

```go
// Mount mounts m's operations on a transport's registry: every operation gets its canonical name Operation(entity, action),
// Requires(Resource(entity.ModelName()), action) and the validation of validate.go. An action in public is mounted Public
// instead (only the app passes public). pub receives the events crud publishes after each commit; nil means nobody listens.
// Before and after mounting it checks the module against its own Definition: if they disagree the generated code is out of
// date (the user forgot to run definitionc) and the first mistake is returned; the transport must not serve.
//
//	if err := crud.Mount(reg, nil, broker, patients); err != nil { return err }
func Mount(reg router.OperationRegistry, public []Public, pub events.Publisher, m Module) error {
	def := m.Definition()
	if err := definitioncheck.CheckModule(def); err != nil {
		return err
	}
	if def.Name != m.ModelName() {
		return &outOfDateError{module: m.ModelName(), entity: def.Name, what: whatName}
	}
	r := &operationMounter{reg: reg, public: public, pub: pub, module: m.ModelName()}
	m.MountOperations(r)
	if r.err != nil {
		return r.err
	}
	return r.verify(def)
}

// verify compares what MountOperations mounted with the entities Definition lists. An entity with Allow == 0 is internal to
// the module and is never mounted.
func (r *operationMounter) verifyDefinition(def model.ModuleDefinition) error {
	for _, h := range def.Entities {
		if h.Allow != 0 && !r.has(h.Name) {
			return &outOfDateError{module: r.module, entity: h.Name, what: whatMissing}
		}
	}
	for _, name := range r.mountedEntities {
		if !isOffered(def, name) {
			return &outOfDateError{module: r.module, entity: name, what: whatExtra}
		}
	}
	return nil
}

func (r *operationMounter) hasMounted(name string) bool {
	for _, m := range r.mountedEntities {
		if m == name {
			return true
		}
	}
	return false
}

// offered reports whether def lists an entity called name that offers any action.
func isOffered(def model.ModuleDefinition, name string) bool {
	for _, h := range def.Entities {
		if h.Name == name && h.Allow != 0 {
			return true
		}
	}
	return false
}
```

## 4.3 `read.go` — `Registry.AddReadOnlyEntity` records what it mounted

In `func (r *operationMounter) Read(...)`, after the two `fail` checks and before `h := func(ctx router.Context) {`, add:

```go
	r.mountedEntities = append(r.mountedEntities, m.ModelName())
```

(`callerOf` became `currentUserOf` in stage 0.)

## 4.4 `loopback.go` — the publisher is a parameter

```go
// Loopback is an in-process router.Caller over real modules, for demos, offline apps and tests: same operation names as rpc
// and mcp ("<ModelName>.<Operation(entity, action)>"), no transport and no RBAC. pub receives the events crud publishes after each
// commit (nil: nobody listens). It panics on a wiring mistake (crud.Mount's error) — it only runs in demos and tests.
//
//	caller := crud.Loopback(nil, patients)
func Loopback(pub events.Publisher, mods ...Module) router.Caller { return newLoopbackCaller(pub, "", mods) }

// LoopbackActingAs is Loopback signed in as userID: operations see it in CurrentUser.UserID.
//
//	caller := crud.LoopbackActingAs(broker, "u1", payments)
func LoopbackActingAs(pub events.Publisher, userID string, mods ...Module) router.Caller {
	return newLoopbackCaller(pub, userID, mods)
}

func newLoopbackCaller(pub events.Publisher, userID string, mods []Module) router.Caller {
```

and inside, replace `Mount(reg, nil, m)` with `Mount(reg, nil, pub, m)` (import `webtyp.com/events`). Nothing else in `loopback.go` changes.

## 4.5 Run

`gotest` must pass with **every** test of stage 1 green. If `TestMount_*` fails because a module fixture lacks `Definition()`, the fixture is wrong (stage 1.2), not
the code. Commit: `refactor!: Mount checks Definition against what was mounted`.
