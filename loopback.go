package crud

import (
	"webtyp.com/fmt"
	"webtyp.com/json"
	"webtyp.com/model"
	"webtyp.com/router"
)

// Loopback is an in-process router.Caller over real modules, for demos, offline apps and tests:
// same operation names as rpc and mcp ("<ModelName>.<Op(entity, action)>"), no transport and
// no RBAC. It panics on a wiring mistake (crud.Mount's error) — it only runs in demos and tests.
//
//	caller := crud.Loopback(patients)
func Loopback(mods ...Module) router.Caller { return newLoopback("", mods) }

// LoopbackActingAs is Loopback signed in as userID: operations see it in Caller.UserID.
//
//	caller := crud.LoopbackActingAs("u1", payments)
func LoopbackActingAs(userID string, mods ...Module) router.Caller { return newLoopback(userID, mods) }

func newLoopback(userID string, mods []Module) router.Caller {
	reg := &loopReg{}
	for _, m := range mods {
		if m == nil {
			continue
		}
		name := m.ModelName()
		if name == "" {
			panic("crud: loopback: a module returned an empty ModelName() — every operation must be qualified by its owning module")
		}
		reg.module = name
		if err := Mount(reg, nil, m); err != nil {
			panic(err.Error())
		}
	}
	return &loopCaller{reg: reg, userID: userID}
}

type loopReg struct {
	ops    []loopOp
	module string
}

type loopOp struct {
	name string
	h    router.HandlerFunc
}

func (r *loopReg) Operation(name string, h router.HandlerFunc) router.Route {
	qualified := r.module + "." + name
	r.ops = append(r.ops, loopOp{name: qualified, h: h})
	return loopRoute{}
}

func (r *loopReg) find(name string) (loopOp, bool) {
	for i := range r.ops {
		if r.ops[i].name == name {
			return r.ops[i], true
		}
	}
	return loopOp{}, false
}

var _ router.OperationRegistry = (*loopReg)(nil)

type loopRoute struct{}

func (r loopRoute) Requires(_ model.Resource, _ model.Action) router.Route { return r }
func (r loopRoute) Authenticated() router.Route                            { return r }
func (r loopRoute) Public() router.Route                                   { return r }
func (r loopRoute) RequiresMFA() router.Route                              { return r }
func (r loopRoute) Accepts(_ model.Fielder) router.Route                   { return r }
func (r loopRoute) Describe(_ string) router.Route                        { return r }

var _ router.Route = loopRoute{}

type loopCaller struct {
	reg    *loopReg
	userID string
}

func (c *loopCaller) Call(op string, args model.Encodable, into model.Decodable, done func(err error)) {
	entry, ok := c.reg.find(op)
	if !ok {
		if done != nil {
			done(fmt.Err("loopback", "unknown", "operation", op))
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

	ctx := &loopCtx{body: buf, userID: c.userID}
	entry.h(ctx)

	if ctx.status >= 400 {
		if done != nil {
			done(newLoopError(ctx.status, ctx.out))
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

func (c *loopCaller) Dispatch(op string, args model.Encodable) {
	c.Call(op, args, nil, nil)
}

var _ router.Caller = (*loopCaller)(nil)

type loopCtx struct {
	body      []byte
	out       []byte
	status    int
	values    []fmt.KeyValue
	userID    string
	assurance model.Assurance
}

func (c *loopCtx) Method() string              { return "" }
func (c *loopCtx) Path() string                { return "" }
func (c *loopCtx) Body() []byte                { return c.body }
func (c *loopCtx) GetHeader(_ string) string   { return "" }
func (c *loopCtx) SetHeader(_, _ string)       {}
func (c *loopCtx) WriteStatus(code int)        { c.status = code }
func (c *loopCtx) Write(b []byte) (int, error) { c.out = append(c.out, b...); return len(b), nil }
func (c *loopCtx) SetValue(key, value string) {
	c.values = append(c.values, fmt.KeyValue{Key: key, Value: value})
}
func (c *loopCtx) Param(_ string) string                 { return "" }
func (c *loopCtx) SetCookie(_ router.Cookie)             {}
func (c *loopCtx) Cookie(_ string) (router.Cookie, bool) { return router.Cookie{}, false }
func (c *loopCtx) SetUserID(id string)                   { c.userID = id }
func (c *loopCtx) UserID() string                        { return c.userID }

func (c *loopCtx) SetAssurance(a model.Assurance) { c.assurance = a }
func (c *loopCtx) Assurance() model.Assurance {
	if c.userID == "" {
		return model.AssuranceNone
	}
	return c.assurance
}

func (c *loopCtx) Value(key string) string {
	for i := range c.values {
		if c.values[i].Key == key {
			return c.values[i].Value
		}
	}
	return ""
}

func (c *loopCtx) Decode(into model.Decodable) error {
	return json.Decode(c.Body(), into)
}

func (c *loopCtx) Encode(v model.Encodable) error {
	var out []byte
	if err := json.Encode(v, &out); err != nil {
		return err
	}
	_, err := c.Write(out)
	return err
}

var _ router.Context = (*loopCtx)(nil)

type loopError struct {
	status int
	body   string
	p      router.Problem
}

func newLoopError(status int, out []byte) error {
	e := &loopError{status: status, body: string(out)}
	if len(out) > 0 && out[0] == '{' {
		_ = json.Decode(out, &e.p)
	}
	return e
}

func (e *loopError) Error() string {
	text := e.p.Detail
	if text == "" {
		text = e.body
	}
	return fmt.Sprintf("loopback: %d %s", e.status, text)
}

func (e *loopError) Kind() model.ErrorKind { return router.KindOfStatus(e.status) }
func (e *loopError) FailedField() string   { return e.p.Field }
