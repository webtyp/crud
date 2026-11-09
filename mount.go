package crud

import (
	"webtyp.com/events"
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
func Mount(reg router.OperationRegistry, public []Public, m Module) error {
	r := &operationMounter{reg: reg, public: public}
	m.MountOperations(r)
	return r.err
}

type operationMounter struct {
	reg             router.OperationRegistry
	public          []Public
	pub             events.Publisher // nil: nobody listens
	module          string           // the module's ModelName()
	mountedEntities []string         // every entity mounted by AddStoredEntity or AddReadOnlyEntity, in order
	err             error
}

func (r *operationMounter) failMount(err error) Route {
	if r.err == nil {
		r.err = err
	}
	return failedRoute{}
}

func (r *operationMounter) isPublicAction(name string, action model.Action) bool {
	for _, p := range r.public {
		if p.Model.ModelName() == name && p.Actions.Has(action) {
			return true
		}
	}
	return false
}

// mountedRoute wraps router.Route so the module only sees RequiresMFA and Describe.
type mountedRoute struct{ r router.Route }

func (x mountedRoute) RequiresMFA() Route         { x.r.RequiresMFA(); return x }
func (x mountedRoute) Describe(text string) Route { x.r.Describe(text); return x }

// failedRoute is returned after a wiring mistake; Mount reports the error.
type failedRoute struct{}

func (failedRoute) RequiresMFA() Route    { return failedRoute{} }
func (failedRoute) Describe(string) Route { return failedRoute{} }

func currentUserOf(ctx router.Context) CurrentUser { return CurrentUser{UserID: ctx.UserID()} }
