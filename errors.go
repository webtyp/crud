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
	ErrServerOnlyField  model.Invalid = "crud: field written only by the server"
)

// ruleError is a mistake of a domain rule: a bug in the module, not in the request. It is not a model.Invalid, so the
// caller sees a generic internal error and the detail goes to the log.
type ruleError string

func (e ruleError) Error() string { return string(e) }

const ErrNoAnswer ruleError = "crud: the rule answered nothing; return crud.ApplyAndPublish or crud.ApplyWithoutPublishing with a nil error, or return an error"
