package crud

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/rpc"
)

// UpdateArgs is what a client sends to patch fields (and what MountUpdate decodes on the server):
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

// MountUpdate registers Op(sample, model.Update) as the field-patch operation of an entity:
// it reads the ids, field names and carrier record sent in {"ids":[…],"fields":[…],"record":{…}}
// and calls update(ids, fields, rec) once. No ids or no fields → 400.
//
//	crud.MountUpdate(reg, func() model.Model { return &Device{} }, func(ids, fields []string, rec model.Model) error {
//		return m.UpdateDevices(ids, fields, *rec.(*Device))
//	})
func MountUpdate(reg router.OperationRegistry, newRecord func() model.Model, update func(ids, fields []string, rec model.Model) error) router.Route {
	sample := newRecord()
	schema := sample.Schema()
	return mountOp(reg, sample, model.Update, sample, func(ctx router.Context) {
		in := UpdateArgs{Record: newRecord()}
		if err := ctx.Decode(&in); err != nil {
			rpc.WriteError(ctx, ErrCannotRead)
			return
		}
		if err := validateIDs(in.IDs, in.hasEmpty, in.tooMany); err != nil {
			rpc.WriteError(ctx, err)
			return
		}
		if err := validateUpdateFields(schema, in.Fields, in.Record); err != nil {
			rpc.WriteError(ctx, err)
			return
		}
		if err := update(in.IDs, in.Fields, in.Record); err != nil {
			rpc.WriteError(ctx, err)
			return
		}
		ctx.WriteStatus(statusNoContent)
	})
}
