# crud
<img src="docs/img/badges.svg">

Standard create, read, update, and delete operation engine for webtyp domain modules: [`crud.Module`](contract.go), [`crud.Registry`](contract.go), [`crud.StoredEntity`](contract.go), [`crud.CurrentUser`](contract.go), and [`crud.Loopback`](loopback.go).

## Why This Exists

In distributed services and multi-tenant architectures, hand-wiring persistence and event publishing leads to inconsistent data and ghost events:

| Without `crud` | With `crud` |
|---|---|
| Each module wrote hand-crafted database mutations and manual `publish` calls; failures could persist without announcing or announce rolled-back data | Domain rules only answer `crud.RuleAnswer` (`ApplyAndPublish` or `ApplyWithoutPublishing`); `crud` commits before publishing |
| Transports and callers accessed raw database tables or forged client-supplied tenant identifiers | `crud` validates schema, boundaries, and passes verified identity via [`crud.CurrentUser`](contract.go) |
| Generated code drifted silently from `definition.go` specifications | [`crud.Mount`](mount.go) compares registered entities and rules against [`model.ModuleDefinition`](contract.go) at startup |

## Before You Start

- Models implement [`model.Model`](contract.go) and declare permissions via [`model.Allower`](contract.go).
- Domain rules return [`crud.RuleAnswer`](result.go) (`crud.ApplyAndPublish` or `crud.ApplyWithoutPublishing`).
- Event subscriptions listen on canonical topics formatted by [`crud.Topic(module, entity, action)`](topic.go) with [`*crud.Change`](change.go) payloads.
- Transports address operations via [`router.Operation{Module, Entity, Action}`](loopback.go).
- A `Store` implementation proves itself with `conformance.Run`: round trips, rollback, a duplicate key, a repeated unique column, a conditional update (`Revision`) and eight goroutines racing for one key, one unique value and one revision — exactly one wins. The factory creates a table for every model of `conformance.Models()`.

## Step-by-Step Guide

### 1. Register a StoredEntity with Business Rules

```go
package example

import (
	"webtyp.com/crud"
	"webtyp.com/events"
	"webtyp.com/model"
	"webtyp.com/router"
)

type Patient struct {
	ID   string
	Name string
}

func (p *Patient) ModelName() string        { return "patient" }
func (p *Patient) Allow() model.Action      { return model.Create | model.Update }
func (p *Patient) Schema() []model.Field    { return []model.Field{{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}}, {Name: "name", Type: model.Text()}} }
func (p *Patient) Pointers() []any          { return []any{&p.ID, &p.Name} }
func (p *Patient) IsNil() bool              { return p == nil }
func (p *Patient) EncodeFields(w model.FieldWriter) { w.String("id", p.ID); w.String("name", p.Name) }
func (p *Patient) DecodeFields(r model.FieldReader) { p.ID, _ = r.String("id"); p.Name, _ = r.String("name") }

type PatientList []Patient
func (l *PatientList) Append() model.Fielder { *l = append(*l, Patient{}); return &(*l)[len(*l)-1] }
func (l *PatientList) Len() int             { return len(*l) }
func (l *PatientList) At(i int) model.Fielder { return &(*l)[i] }
func (l *PatientList) IsNil() bool          { return l == nil }
func (l *PatientList) EncodeFields(w model.FieldWriter) {}
func (l *PatientList) DecodeFields(r model.FieldReader) {}

type PatientModule struct {
	Store crud.Store
}

func (m *PatientModule) ModelName() string { return "patients" }
func (m *PatientModule) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{
		Name: "patients",
		Entities: []model.Definition{{
			Name:  "patient",
			Allow: model.Create | model.Update,
			Rules: model.Rules{Create: model.AddMyBusinessRule, Update: model.HaveNoBusinessRule},
		}},
	}
}

func (m *PatientModule) MountOperations(reg crud.Registry) {
	reg.AddStoredEntity(crud.StoredEntity{
		Model:   &Patient{},
		NewList: func() model.ModelSlice { return &PatientList{} },
		Store:   m.Store,
		Rules:   model.Rules{Create: model.AddMyBusinessRule, Update: model.HaveNoBusinessRule},
		Create: func(u crud.CurrentUser, tx crud.Tx, rec model.Model) (crud.RuleAnswer, error) {
			p := rec.(*Patient)
			if p.Name == "" {
				return 0, model.Conflict("name required")
			}
			return crud.ApplyAndPublish, nil
		},
	})
}

func MountModule(reg router.OperationRegistry, broker events.Publisher, mod *PatientModule) error {
	return crud.Mount(reg, nil, broker, mod)
}
```

### 2. Subscribe to Published Changes

```go
func SubscribePatients(sub events.Subscriber) {
	topic := crud.Topic("patients", "patient", model.Create)
	sub.Subscribe(topic, func(e events.Event) {
		if ch, ok := e.Payload.(*crud.Change); ok {
			p := ch.Record.(*Patient)
			_ = p.ID
		}
	})
}
```

## Documentation Map

| Document | Purpose |
|---|---|
| [`contract.go`](contract.go) | Public interfaces: `Module`, `Registry`, `Route`, `CurrentUser`, `StoredEntity`, `Store`, `Tx` |
| [`stored_entity.go`](stored_entity.go) | Engine mounting `StoredEntity` operations and verifying declarations |
| [`stored_entity_create.go`](stored_entity_create.go) | Create batch pipeline, transactional persistence, and event announcement |
| [`stored_entity_update.go`](stored_entity_update.go) | Partial field updates and change tracking |
| [`stored_entity_delete.go`](stored_entity_delete.go) | Transactional record deletion and event publishing |
| [`stored_entity_read.go`](stored_entity_read.go) | Record and query resolution with automatic limit capping |
| [`result.go`](result.go) | `RuleAnswer` decision flags (`ApplyAndPublish`, `ApplyWithoutPublishing`) |
| [`change.go`](change.go) | `Change` event payload structure encoding touched fields and record state |
| [`topic.go`](topic.go) | Canonical event topic generation `Topic(module, entity, action)` |
| [`mount.go`](mount.go) | `crud.Mount` operation registration and consistency verification |
| [`loopback.go`](loopback.go) | In-process testing caller `crud.Loopback` and `crud.LoopbackActingAs` |
| [`conformance/`](conformance/) | Conformance suite: a `Store` implementation proves itself with `conformance.Run` |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Architectural layout and request flow |
| [`docs/diagrams/WRITE_FLOW.md`](docs/diagrams/WRITE_FLOW.md) | Visual flow of validation, rule evaluation, commit, and post-commit publishing |
| [`docs/diagrams/PLACEMENT.md`](docs/diagrams/PLACEMENT.md) | Framework placement diagram |
| [`docs/TESTS.md`](docs/TESTS.md) | Domain test matrix |
