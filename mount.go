package crud

import (
	"webtyp.com/definitioncheck"
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

var defaultQueue CallQueue

// SetDefaultQueue sets the default queue used when mounting or calling without an explicit queue.
//
//	crud.SetDefaultQueue(queue)
func SetDefaultQueue(q CallQueue) {
	defaultQueue = q
}

// DefaultQueue answers the default queue set by SetDefaultQueue, or nil when none was set.
//
//	q := crud.DefaultQueue()
func DefaultQueue() CallQueue {
	return defaultQueue
}

// Mount mounts m's operations on a transport's registry: every operation gets its canonical name Operation(entity, action),
// Requires(Resource(entity.ModelName()), action) and the validation of validate.go. An action in public is mounted Public
// instead (only the app passes public). pub receives the events crud publishes after each commit; nil means nobody listens.
// Before and after mounting it checks the module against its own Definition: if they disagree the generated code is out of
// date (the user forgot to run definitionc) and the first mistake is returned; the transport must not serve.
//
//	if err := crud.Mount(reg, nil, broker, patients); err != nil { return err }
func Mount(reg router.OperationRegistry, public []Public, pub events.Publisher, m Module) error {
	return MountWithQueue(reg, public, pub, defaultQueue, m)
}

// MountWithQueue mounts m's operations on a transport's registry, providing q for orchestrating external calls.
//
//	if err := crud.MountWithQueue(reg, nil, broker, queue, payments); err != nil { return err }
func MountWithQueue(reg router.OperationRegistry, public []Public, pub events.Publisher, q CallQueue, m Module) error {
	if q == nil {
		q = defaultQueue
	}
	def := m.Definition()
	if err := definitioncheck.CheckModule(def); err != nil {
		return err
	}
	if def.Name != m.ModelName() {
		return &outOfDateError{module: m.ModelName(), entity: def.Name, what: whatName}
	}
	r := &operationMounter{reg: reg, public: public, pub: pub, queue: q, module: m.ModelName()}
	m.MountOperations(r)
	if r.err != nil {
		return r.err
	}
	return r.verify(def)
}

type operationMounter struct {
	reg             router.OperationRegistry
	public          []Public
	pub             events.Publisher // nil: nobody listens
	queue           CallQueue        // nil: no external calls
	module          string           // the module's ModelName()
	mountedEntities []string         // every entity mounted by AddStoredEntity or AddReadOnlyEntity, in order
	err             error
}

// verify compares what MountOperations mounted with the entities Definition lists. An entity with Allow == 0 is internal to
// the module and is never mounted.
func (r *operationMounter) verify(def model.ModuleDefinition) error {
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

func (r *operationMounter) has(name string) bool {
	for _, m := range r.mountedEntities {
		if m == name {
			return true
		}
	}
	return false
}

// isOffered reports whether def lists an entity called name that offers any action.
func isOffered(def model.ModuleDefinition, name string) bool {
	for _, h := range def.Entities {
		if h.Name == name && h.Allow != 0 {
			return true
		}
	}
	return false
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
