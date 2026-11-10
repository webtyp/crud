package crud

import (
	"webtyp.com/model"
	"webtyp.com/storage"
)

// CurrentUser is who runs the operation, filled by crud from the authenticated session — never from the request body.
// UserID is "" only on an action the app declared public.
//
// Example: func(a crud.CurrentUser, tx crud.Tx, p *Payment, m *Module) (crud.RuleAnswer, error) { p.CreatedBy = a.UserID; … }
type CurrentUser struct {
	UserID string
}

// Route is what a module may still say about one of its operations. It has no Public,
// Authenticated or Requires: crud fixes the permission to Resource(ModelName) + the action,
// and only the app (appkit PublicReads) can open a read to visitors.
type Route interface {
	RequiresMFA() Route
	Describe(text string) Route
}

// Registry is the only thing a module receives in MountOperations: the standard operations.
type Registry interface {
	// AddStoredEntity mounts the actions entity.Model allows, persisted by entity.Store.
	AddStoredEntity(entity StoredEntity)
	// AddReadOnlyEntity mounts a read of a model that is not a table (a report, a derived view), answered by read.
	AddReadOnlyEntity(m model.ModuleNaming, newFilter func() model.Model, read func(c CurrentUser, args ReadArgs, filter model.Model) (model.ModelSlice, error)) Route
}

// StoredEntity is one entity a module offers and crud stores through a Store. definitionc builds it from the model's Rules and the module's rules; a hook is nil
// exactly when its action is declared model.HaveNoBusinessRule (plain storage) — StoredEntity checks this at mount.
type StoredEntity struct {
	Model   model.Model // a zero record; must implement model.Allower with Allow() != 0
	NewList func() model.ModelSlice
	Store   Store
	// Rules is the model's declared Rules (PatientModel.Rules). A AddMyBusinessRule action needs its hook, a HaveNoBusinessRule action must have none.
	Rules model.Rules
	// Create runs per record, before the insert, inside the transaction. It answers whether to announce (crud.ApplyAndPublish or crud.ApplyWithoutPublishing).
	Create func(a CurrentUser, tx Tx, rec model.Model) (RuleAnswer, error)
	// Update runs per id; old is the stored row and next carries only fields.
	Update func(a CurrentUser, tx Tx, old, next model.Model, fields []string) (RuleAnswer, error)
	// Delete runs per id, before the delete; rec is the stored row.
	Delete func(a CurrentUser, tx Tx, rec model.Model) (RuleAnswer, error)
	// Search replaces the default search: AND-only conditions. Declared with Rules.Read == model.AddMyBusinessRule.
	// It runs on every read of the table, with or without text. filter is a record of the entity itself carrying only the fields the
	// caller sent (the entity is its own filter: `filter.sku` on the wire); text is "" when none was sent.
	Search func(a CurrentUser, filter model.Model, text string) ([]storage.Condition, error)
	MFA    model.Action // actions that require a second factor
}

// Public is one action the app opens to visitors without a session (reads and writes). Only the app lists them.
//
//	Public: []crud.Public{{Model: &abook.OnlineReservation{}, Actions: model.Create}}
type Public struct {
	Model   model.ModuleNaming
	Actions model.Action
}

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
