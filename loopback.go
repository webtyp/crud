package crud

import (
	"webtyp.com/events"
	"webtyp.com/fmt"
	"webtyp.com/json"
	"webtyp.com/model"
	"webtyp.com/router"
)

// Loopback is an in-process router.Caller over real modules, for demos, offline apps and tests:
// operations are addressed by router.Operation, no transport and no RBAC. pub receives the events crud publishes after each
// commit (nil: nobody listens). It panics on a wiring mistake (crud.Mount's error) — it only runs in demos and tests.
//
//	caller := crud.Loopback(nil, patients)
func Loopback(pub events.Publisher, mods ...Module) router.Caller {
	return LoopbackWithQueue(pub, defaultQueue, mods...)
}

// LoopbackWithQueue is Loopback with a CallQueue for orchestrating external calls.
//
//	caller := crud.LoopbackWithQueue(broker, queue, payments)
func LoopbackWithQueue(pub events.Publisher, q CallQueue, mods ...Module) router.Caller {
	if q == nil {
		q = defaultQueue
	}
	return newLoopbackCaller(pub, "", q, mods)
}

// LoopbackActingAs is Loopback signed in as userID: operations see it in CurrentUser.UserID.
//
//	caller := crud.LoopbackActingAs(broker, "u1", payments)
func LoopbackActingAs(pub events.Publisher, userID string, mods ...Module) router.Caller {
	return LoopbackActingAsWithQueue(pub, userID, defaultQueue, mods...)
}

// LoopbackActingAsWithQueue is LoopbackActingAs with a CallQueue for orchestrating external calls.
//
//	caller := crud.LoopbackActingAsWithQueue(broker, "u1", queue, payments)
func LoopbackActingAsWithQueue(pub events.Publisher, userID string, q CallQueue, mods ...Module) router.Caller {
	if q == nil {
		q = defaultQueue
	}
	return newLoopbackCaller(pub, userID, q, mods)
}

func newLoopbackCaller(pub events.Publisher, userID string, q CallQueue, mods []Module) router.Caller {
	reg := &loopbackRegistry{}
	for _, m := range mods {
		if m == nil {
			continue
		}
		name := m.ModelName()
		if name == "" {
			panic("crud: loopback: a module returned an empty ModelName() — every operation must be qualified by its owning module")
		}
		if err := MountWithQueue(reg, nil, pub, q, m); err != nil {
			panic(err.Error())
		}
	}
	return &loopbackCaller{reg: reg, userID: userID}
}

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

var _ router.OperationRegistry = (*loopbackRegistry)(nil)

type loopbackRoute struct{}

func (r loopbackRoute) Requires(_ model.Resource, _ model.Action) router.Route { return r }
func (r loopbackRoute) Authenticated() router.Route                            { return r }
func (r loopbackRoute) Public() router.Route                                   { return r }
func (r loopbackRoute) RequiresMFA() router.Route                              { return r }
func (r loopbackRoute) Accepts(_ model.Fielder) router.Route                   { return r }
func (r loopbackRoute) Describe(_ string) router.Route                         { return r }

var _ router.Route = loopbackRoute{}

type loopbackCaller struct {
	reg    *loopbackRegistry
	userID string
}

func (c *loopbackCaller) Call(op router.Operation, args model.Encodable, into model.Decodable, done func(err error)) {
	entry, ok := c.reg.find(op)
	if !ok {
		if done != nil {
			done(fmt.Err("loopback", "unknown", "operation", op.String()))
		}
		return
	}

	var buf []byte
	if args == nil || model.IsNil(args) {
		buf = []byte("{}")
	} else if err := json.Encode(args, &buf); err != nil {
		if done != nil {
			done(err)
		}
		return
	}

	ctx := &loopbackContext{body: buf, userID: c.userID}
	entry.h(ctx)

	if ctx.status >= 400 {
		if done != nil {
			done(newLoopbackError(ctx.status, ctx.out))
		}
		return
	}

	if into != nil && len(ctx.out) > 0 {
		if err := json.Decode(ctx.out, into); err != nil {
			if done != nil {
				done(err)
			}
			return
		}
	}
	if done != nil {
		done(nil)
	}
}

func (c *loopbackCaller) Dispatch(op router.Operation, args model.Encodable) {
	c.Call(op, args, nil, nil)
}

var _ router.Caller = (*loopbackCaller)(nil)

var _ router.Context = (*loopbackContext)(nil)

type loopbackContext struct {
	body      []byte
	out       []byte
	status    int
	values    []fmt.KeyValue
	userID    string
	assurance model.Assurance
}

func (c *loopbackContext) Method() string            { return "" }
func (c *loopbackContext) Path() string              { return "" }

// RawQuery is empty: a loopback call has no URL, its arguments travel in the body.
func (c *loopbackContext) RawQuery() string { return "" }

func (c *loopbackContext) Body() []byte              { return c.body }
func (c *loopbackContext) GetHeader(_ string) string { return "" }
func (c *loopbackContext) SetHeader(_, _ string)     {}
func (c *loopbackContext) WriteStatus(code int)      { c.status = code }
func (c *loopbackContext) Write(b []byte) (int, error) {
	c.out = append(c.out, b...)
	return len(b), nil
}
func (c *loopbackContext) SetValue(key, value string) {
	c.values = append(c.values, fmt.KeyValue{Key: key, Value: value})
}
func (c *loopbackContext) Param(_ string) string                 { return "" }
func (c *loopbackContext) SetCookie(_ router.Cookie)             {}
func (c *loopbackContext) Cookie(_ string) (router.Cookie, bool) { return router.Cookie{}, false }
func (c *loopbackContext) SetUserID(id string)                   { c.userID = id }
func (c *loopbackContext) UserID() string                        { return c.userID }

func (c *loopbackContext) SetAssurance(a model.Assurance) { c.assurance = a }
func (c *loopbackContext) Assurance() model.Assurance {
	if c.userID == "" {
		return model.AssuranceNone
	}
	return c.assurance
}

func (c *loopbackContext) Value(key string) string {
	for i := range c.values {
		if c.values[i].Key == key {
			return c.values[i].Value
		}
	}
	return ""
}

func (c *loopbackContext) Decode(into model.Decodable) error {
	return json.Decode(c.Body(), into)
}

func (c *loopbackContext) Encode(v model.Encodable) error {
	var out []byte
	if err := json.Encode(v, &out); err != nil {
		return err
	}
	_, err := c.Write(out)
	return err
}

var _ router.Context = (*loopbackContext)(nil)

type loopbackError struct {
	status int
	body   string
	p      router.Problem
}

func newLoopbackError(status int, out []byte) error {
	e := &loopbackError{status: status, body: string(out)}
	if len(out) > 0 && out[0] == '{' {
		_ = json.Decode(out, &e.p)
	}
	return e
}

func (e *loopbackError) Error() string {
	text := e.p.Detail
	if text == "" {
		text = e.body
	}
	return fmt.Sprintf("loopback: %d %s", e.status, text)
}

func (e *loopbackError) Kind() model.ErrorKind { return router.KindOfStatus(e.status) }
func (e *loopbackError) FailedField() string   { return e.p.Field }
