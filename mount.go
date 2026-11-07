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

func (x route) RequiresMFA() Route         { x.r.RequiresMFA(); return x }
func (x route) Describe(text string) Route { x.r.Describe(text); return x }

// noRoute is returned after a wiring mistake; Mount reports the error.
type noRoute struct{}

func (noRoute) RequiresMFA() Route    { return noRoute{} }
func (noRoute) Describe(string) Route { return noRoute{} }

func callerOf(ctx router.Context) Caller { return Caller{UserID: ctx.UserID()} }
