package crud

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/rpc"
)

// CreateArgs is what a client sends to a create operation: whole records, persisted in one transactional batch.
//
// Example: caller.Call(crud.Op(&Device{}, model.Create), &crud.CreateArgs{Records: recs}, saved, done)
type CreateArgs struct{ Records []model.Model }

func (a *CreateArgs) IsNil() bool { return a == nil }

func (a *CreateArgs) EncodeFields(w model.FieldWriter) {
	arr := w.Array(keyRecords, len(a.Records))
	for _, r := range a.Records {
		arr.Object(r)
	}
}

type createIn struct {
	list    model.ModelSlice
	tooMany bool
}

func (c *createIn) IsNil() bool { return c == nil }

func (c *createIn) DecodeFields(r model.FieldReader) {
	arr, ok := r.Array(keyRecords)
	if !ok {
		return
	}
	n := arr.Len()
	if n > MaxPage {
		c.tooMany = true
		return
	}
	for i := 0; i < n; i++ {
		elem, ok := c.list.Append().(model.Decodable)
		if !ok {
			return
		}
		arr.Object(i, elem)
	}
}

// MountCreate registers Op(sample, model.Create) as the batch creation operation of an entity:
// it decodes all records sent in {"records":[…]} into a fresh ModelSlice from newList, invokes
// create(list) once so the module can persist the entire batch in a single transaction, and
// answers the last record in the batch (create may fill its id).
// No records → 400; a create error → its own status.
//
//	crud.MountCreate(reg, func() model.ModelSlice { return &DeviceList{} }, func(list model.ModelSlice) error {
//		return m.CreateDevices(*list.(*DeviceList))
//	})
func MountCreate(reg router.OperationRegistry, newList func() model.ModelSlice, create func(list model.ModelSlice) error) router.Route {
	sample := newList().Append().(model.Model)
	return mountOp(reg, sample, model.Create, sample, func(ctx router.Context) {
		in := createIn{list: newList()}
		if err := ctx.Decode(&in); err != nil {
			rpc.WriteError(ctx, ErrCannotRead)
			return
		}
		if err := validateCreateBatch(in.list, in.tooMany); err != nil {
			rpc.WriteError(ctx, err)
			return
		}
		if err := create(in.list); err != nil {
			rpc.WriteError(ctx, err)
			return
		}
		last := in.list.At(in.list.Len() - 1).(model.Encodable)
		if err := ctx.Encode(last); err != nil {
			rpc.WriteError(ctx, err)
		}
	})
}
