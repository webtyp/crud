package crud

import (
	"webtyp.com/model"
	"webtyp.com/router"
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

func (r *registry) Create(newList func() model.ModelSlice, create func(c Caller, list model.ModelSlice) error) Route {
	if newList == nil || create == nil {
		return r.fail(ErrNilFunction)
	}
	sample := newList().Append().(model.Model)
	if sample.ModelName() == "" {
		return r.fail(ErrEmptyModelName)
	}
	schema := sample.Schema()
	h := func(ctx router.Context) {
		in := createIn{list: newList()}
		if err := ctx.Decode(&in); err != nil {
			router.WriteError(ctx, ErrCannotRead)
			return
		}
		if err := validateCreateBatch(schema, in.list, in.tooMany); err != nil {
			router.WriteError(ctx, err)
			return
		}
		if err := create(callerOf(ctx), in.list); err != nil {
			router.WriteError(ctx, err)
			return
		}
		last := in.list.At(in.list.Len() - 1).(model.Encodable)
		if err := ctx.Encode(last); err != nil {
			router.WriteError(ctx, err)
		}
	}
	rt := r.reg.Operation(Op(sample, model.Create), h).Requires(model.Resource(sample.ModelName()), model.Create)
	rt.Accepts(sample)
	return route{rt}
}
