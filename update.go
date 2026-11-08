package crud

import (
	"webtyp.com/model"
)

// UpdateArgs is what a client sends to patch fields (and what Update decodes on the server):
// which rows, which columns, and a record carrying the values.
//
// Example: caller.Call(crud.Op(&Device{}, model.Update), &crud.UpdateArgs{IDs: ids, Fields: fields, Record: rec}, nil, done)
type UpdateArgs struct {
	IDs      []string
	Fields   []string
	Record   model.Model
	oneID    [1]string
	oneField [1]string
	tooMany  bool
	hasEmpty bool
}

func (a *UpdateArgs) IsNil() bool { return a == nil }

func (a *UpdateArgs) EncodeFields(w model.FieldWriter) {
	ids := w.Array(keyIDs, len(a.IDs))
	for _, id := range a.IDs {
		ids.String(id)
	}
	fields := w.Array(keyFields, len(a.Fields))
	for _, f := range a.Fields {
		fields.String(f)
	}
	w.Object(keyRecord, a.Record)
}

func (a *UpdateArgs) DecodeFields(r model.FieldReader) {
	a.IDs, a.tooMany, a.hasEmpty = decodeStrings(r, keyIDs, &a.oneID)
	if a.tooMany {
		return
	}
	var fTooMany bool
	a.Fields, fTooMany, _ = decodeStrings(r, keyFields, &a.oneField)
	if fTooMany {
		a.tooMany = true
		return
	}
	if a.Record != nil {
		r.Object(keyRecord, a.Record)
	}
}

