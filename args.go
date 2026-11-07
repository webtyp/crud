package crud

import "webtyp.com/model"

const (
	keyRecords = "records"
	keyIDs     = "ids"
	keyFields  = "fields"
	keyRecord  = "record"
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
	a.IDs = decodeStrings(r, keyIDs, &a.oneID)
	a.Fields = decodeStrings(r, keyFields, &a.oneField)
	if a.Record != nil {
		r.Object(keyRecord, a.Record)
	}
}

// DeleteArgs is what a client sends to a delete operation (and what MountDelete decodes on the server):
// the ids to remove.
//
// Example: caller.Call(crud.Op(&Device{}, model.Delete), &crud.DeleteArgs{IDs: ids}, nil, done)
type DeleteArgs struct {
	IDs []string
	one [1]string
}

func (a *DeleteArgs) IsNil() bool { return a == nil }

func (a *DeleteArgs) EncodeFields(w model.FieldWriter) {
	ids := w.Array(keyIDs, len(a.IDs))
	for _, id := range a.IDs {
		ids.String(id)
	}
}

func (a *DeleteArgs) DecodeFields(r model.FieldReader) {
	a.IDs = decodeStrings(r, keyIDs, &a.one)
}

type createIn struct {
	list model.ModelSlice
}

func (c *createIn) IsNil() bool { return c == nil }

func (c *createIn) DecodeFields(r model.FieldReader) {
	arr, ok := r.Array(keyRecords)
	if !ok {
		return
	}
	n := arr.Len()
	for i := 0; i < n; i++ {
		elem, ok := c.list.Append().(model.Decodable)
		if !ok {
			return
		}
		arr.Object(i, elem)
	}
}

func decodeStrings(r model.FieldReader, key string, one *[1]string) []string {
	arr, ok := r.Array(key)
	if !ok {
		return nil
	}
	n := arr.Len()
	if n == 0 {
		return nil
	}
	if n == 1 {
		one[0] = arr.String(0)
		return one[:1]
	}
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = arr.String(i)
	}
	return out
}
