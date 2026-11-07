package crud

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/rpc"
)

// DeleteArgs is what a client sends to a delete operation (and what MountDelete decodes on the server):
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

// MountDelete registers Op(m, model.Delete) as the delete operation of an entity:
// it reads the ids sent in {"ids":[…]} and calls remove(ids) once. No ids → 400.
//
//	crud.MountDelete(reg, &Device{}, m.DeleteDevices)
func MountDelete(reg router.OperationRegistry, m model.ModuleNaming, remove func(ids []string) error) router.Route {
	return mountOp(reg, m, model.Delete, nil, func(ctx router.Context) {
		var in DeleteArgs
		if err := ctx.Decode(&in); err != nil {
			rpc.WriteError(ctx, ErrCannotRead)
			return
		}
		if err := validateIDs(in.IDs, in.hasEmpty, in.tooMany); err != nil {
			rpc.WriteError(ctx, err)
			return
		}
		if err := remove(in.IDs); err != nil {
			rpc.WriteError(ctx, err)
			return
		}
		ctx.WriteStatus(statusNoContent)
	})
}
