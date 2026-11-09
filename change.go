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
