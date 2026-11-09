package crud

import (
	"webtyp.com/model"
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

type createRequest struct {
	list    model.ModelSlice
	tooMany bool
}

func (c *createRequest) IsNil() bool { return c == nil }

func (c *createRequest) DecodeFields(r model.FieldReader) {
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

