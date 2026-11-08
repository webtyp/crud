package crud

import (
	"webtyp.com/model"
	"webtyp.com/storage"
)

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

// Registry is the only thing a module receives in MountOperations: the standard operations.
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

// Public is one action the app opens to visitors without a session (reads and writes). Only the app lists them.
//
//	Public: []crud.Public{{Model: &abook.OnlineReservation{}, Actions: model.Create}}
type Public struct {
	Model   model.ModuleNaming
	Actions model.Action
}

// Module is a domain module: it names itself and mounts its entities' operations.
//
//	var _ crud.Module = (*Module)(nil)
type Module interface {
	model.ModuleNaming
	MountOperations(reg Registry)
}
