package crud

import "webtyp.com/model"

const (
	ErrNoRecords        model.Invalid = "crud: no records to save"
	ErrNoIDs            model.Invalid = "crud: no ids given"
	ErrNoFields         model.Invalid = "crud: no fields given"
	ErrCannotRead       model.Invalid = "crud: cannot read the request"
	ErrBatchTooLarge    model.Invalid = "crud: batch size exceeds limit"
	ErrInvalidID        model.Invalid = "crud: empty id"
	ErrInvalidField     model.Invalid = "crud: immutable field"
	ErrUnknownField     model.Invalid = "crud: unknown field"
	ErrArgTooLong       model.Invalid = "crud: argument too long"
	ErrServerOnlyField  model.Invalid  = "crud: field written only by the server"
	ErrAlreadyExists    model.Conflict = "crud: the key already exists"
	ErrConflict         model.Conflict = "crud: the record changed since you read it; read it again"
	ErrRevisionRequired model.Invalid  = "crud: send the revision you read"
)

// ruleError is a mistake of a domain rule: a bug in the module, not in the request. It is not a model.Invalid, so the
// caller sees a generic internal error and the detail goes to the log.
type ruleError string

func (e ruleError) Error() string { return string(e) }

const ErrNoAnswer ruleError = "crud: the rule answered nothing; return crud.ApplyAndPublish or crud.ApplyWithoutPublishing with a nil error, or return an error"

const ErrGeneratedOutOfDate mountError = "crud: the generated code does not match definition.go, so nothing is served; run definitionc and rebuild"

const (
	whatMissing = "is listed in definition.go but was not mounted"
	whatExtra   = "was mounted but is not listed in definition.go"
	whatRules   = "has rules in definition.go that differ from the generated hooks"
	whatName    = "has a Definition name that differs from its ModelName"
)

// outOfDateError is ErrGeneratedOutOfDate with where: the module, the entity and, for rules, the actions as CRUD letters.
type outOfDateError struct {
	module  string
	entity  string
	what    string
	actions model.Action
}

func (e *outOfDateError) Error() string {
	text := string(ErrGeneratedOutOfDate) + " — " + e.module + "." + e.entity + " " + e.what
	if e.actions != 0 {
		text += " (" + e.actions.String() + ")"
	}
	return text
}
