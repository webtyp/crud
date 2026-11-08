package crud

import (
	"webtyp.com/model"
)

// DeleteArgs is what a client sends to a delete operation (and what Delete decodes on the server):
// the ids to remove.
//
// Example: caller.Call(crud.Op(&Device{}, model.Delete), &crud.DeleteArgs{IDs: ids}, nil, done)
type DeleteArgs struct {
	IDs      []string
	one      [1]string
	tooMany  bool
	hasEmpty bool
}

func (a *DeleteArgs) IsNil() bool { return a == nil }

func (a *DeleteArgs) EncodeFields(w model.FieldWriter) {
	ids := w.Array(keyIDs, len(a.IDs))
	for _, id := range a.IDs {
		ids.String(id)
	}
}

func (a *DeleteArgs) DecodeFields(r model.FieldReader) {
	a.IDs, a.tooMany, a.hasEmpty = decodeStrings(r, keyIDs, &a.one)
}

