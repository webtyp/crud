← [Stage 1](PLAN_STAGE_1_TESTS.md) | Next → [Stage 3](PLAN_STAGE_3_HANDLERS.md)

# Stage 2 — the new types, and Caller becomes CurrentUser

Nothing in this stage changes behaviour. After it the repo compiles (tests stay red until stage 4).

## 2.1 `result.go` (new)

```go
package crud

// RuleAnswer is what a rule answers when its business allows the write: store it, and say whether to publish the event.
// The zero value is not an answer: a rule that returns it is refused (ErrNoAnswer).
//
//	return crud.ApplyAndPublish, nil
type RuleAnswer uint8

const (
	// ApplyAndPublish: crud stores the change and, after the commit, publishes it on Topic(module, entity, action).
	ApplyAndPublish RuleAnswer = iota + 1
	// ApplyWithoutPublishing: crud stores the change and publishes nothing.
	ApplyWithoutPublishing
)

// valid reports whether the rule really answered (the zero RuleAnswer did not).
func (r RuleAnswer) valid() bool { return r == ApplyAndPublish || r == ApplyWithoutPublishing }
```

## 2.2 `change.go` (new)

```go
package crud

import "webtyp.com/model"

// Change is the payload of every event crud publishes: the record and, for an update, the fields that were written.
// A false or empty value in Record cannot be told from "not sent", so a subscriber that needs to know what changed reads Fields.
//
//	if ch, ok := e.Payload.(*crud.Change); ok && ch.Touched("is_active") { … }
type Change struct {
	Record model.Model
	Fields []string // the written fields of an update; nil for a create or a delete
}

func (c *Change) IsNil() bool { return c == nil }

// Touched reports whether an update wrote the field.
func (c *Change) Touched(field string) bool {
	for _, f := range c.Fields {
		if f == field {
			return true
		}
	}
	return false
}

// EncodeFields writes the record under "record" and, when there are fields, the array "fields". It does not call Close on the
// array: like CreateArgs.EncodeFields, it writes exactly the length it announced.
func (c *Change) EncodeFields(w model.FieldWriter) {
	w.Object("record", c.Record)
	if len(c.Fields) == 0 {
		return
	}
	arr := w.Array("fields", len(c.Fields))
	for _, f := range c.Fields {
		arr.String(f)
	}
}
```

## 2.3 `topic.go` (new)

```go
package crud

import (
	"webtyp.com/events"
	"webtyp.com/model"
)

const (
	topicSeparator = "."
	verbCreated    = "created"
	verbUpdated    = "updated"
	verbDeleted    = "deleted"
)

// Topic is the name of the event crud publishes for an action: "<module>.<entity>.created|updated|deleted".
// A read has none (""). Subscribers build the name with this function, never with a string of their own.
//
//	sub.Subscribe(crud.Topic("patient_directory", "patient", model.Create), func(e events.Event) { … })
func Topic(module, entity string, a model.Action) string {
	var verb string
	switch a {
	case model.Create:
		verb = verbCreated
	case model.Update:
		verb = verbUpdated
	case model.Delete:
		verb = verbDeleted
	default:
		return ""
	}
	return module + topicSeparator + entity + topicSeparator + verb
}

// eventPublisher publishes the changes of one entity. A nil publisher means nobody listens (a demo, a test, mcp harvest): not an error.
type eventPublisher struct {
	pub    events.Publisher
	module string
	entity string
}

func (e eventPublisher) publish(a model.Action, ch *Change) {
	if e.pub == nil {
		return
	}
	e.pub.Publish(events.Event{Topic: Topic(e.module, e.entity, a), Payload: ch})
}
```

## 2.4 `errors.go` — add

```go
// ruleError is a mistake of a domain rule: a bug in the module, not in the request. It is not a model.Invalid, so the
// caller sees a generic internal error and the detail goes to the log.
type ruleError string

func (e ruleError) Error() string { return string(e) }

const ErrNoAnswer ruleError = "crud: the rule answered nothing; return crud.ApplyAndPublish or crud.ApplyWithoutPublishing with a nil error, or return an error"
```

## 2.5 Rename `Caller` → `CurrentUser` (code, comments and docs of this repo)

In `contract.go` the type becomes:

```go
// CurrentUser is who runs the operation, filled by crud from the authenticated session — never from the request body.
// UserID is "" only on an action the app declared public.
//
// Example: func(a crud.CurrentUser, tx crud.Tx, p *Payment, m *Module) (crud.RuleAnswer, error) { p.CreatedBy = a.UserID; … }
type CurrentUser struct {
	UserID string
}
```

Every `Caller` in `contract.go` (lines 8–42), `read.go:82` and `mount.go:64` becomes `CurrentUser`; `currentUserOf` (renamed from `callerOf` in stage 0) now returns `CurrentUser`; the comment in `loopback.go:17` says `CurrentUser.UserID`.
`router.Caller` and `loopbackCaller` (the in-memory client) are **not** renamed. Verify:

```bash
grep -rn "\bCaller\b" --include=*.go . | grep -v "router.Caller\|tests/"   # → empty
grep -rn "callerOf" --include=*.go .                                       # → empty
```

`gotest` must compile the package (the `tests/` package will not until stage 4). Commit: `feat: RuleAnswer, Change, Topic; Caller becomes CurrentUser`.
