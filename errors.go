package crud

import "webtyp.com/model"

const (
	ErrNoRecords     model.Invalid = "crud: no records to save"
	ErrNoIDs         model.Invalid = "crud: no ids given"
	ErrNoFields      model.Invalid = "crud: no fields given"
	ErrCannotRead    model.Invalid = "crud: cannot read the request"
	ErrBatchTooLarge model.Invalid = "crud: batch size exceeds limit"
	ErrInvalidID     model.Invalid = "crud: empty id"
	ErrInvalidField  model.Invalid = "crud: immutable field"
	ErrUnknownField  model.Invalid = "crud: unknown field"
)
