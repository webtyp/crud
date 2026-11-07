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
