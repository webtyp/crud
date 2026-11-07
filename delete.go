package crud

import (
	"webtyp.com/model"
	"webtyp.com/router"
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

func (r *registry) Delete(m model.ModuleNaming, remove func(c Caller, ids []string) error) Route {
	if m == nil || remove == nil {
		return r.fail(ErrNilFunction)
	}
	if m.ModelName() == "" {
		return r.fail(ErrEmptyModelName)
	}
	h := func(ctx router.Context) {
		var in DeleteArgs
		if err := ctx.Decode(&in); err != nil {
			router.WriteError(ctx, ErrCannotRead)
			return
		}
		if err := validateIDs(in.IDs, in.hasEmpty, in.tooMany); err != nil {
			router.WriteError(ctx, err)
			return
		}
		if err := remove(callerOf(ctx), in.IDs); err != nil {
			router.WriteError(ctx, err)
			return
		}
		ctx.WriteStatus(statusNoContent)
	}
	rt := r.reg.Operation(Op(m, model.Delete), h).Requires(model.Resource(m.ModelName()), model.Delete)
	return route{rt}
}
