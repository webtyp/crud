package crud

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/rpc"
)

// MaxPage is the most rows one read call returns; the server caps every request to it (Cap).
const MaxPage = 500

// MaxSearch is the maximum length in bytes for a search query.
const MaxSearch = 256

// MaxAfter is the maximum length in bytes for a pagination cursor.
const MaxAfter = 128

// MaxID is the maximum length in bytes for a record identifier.
const MaxID = 128

// Cap is the limit a read operation must use: the asked limit, or MaxPage when it is missing or larger.
//
// Example: rows, err := m.readPatients(m.tenantID, args.Search, args.After, crud.Cap(args.Limit))
func Cap(limit int64) int64 {
	if limit <= 0 || limit > MaxPage {
		return MaxPage
	}
	return limit
}

// ReadArgs is what a read operation receives: an optional record ID for single-row lookup,
// the search term ("" = no search), the id of the last row already shown ("" = from the start;
// the server returns the rows after it in its own stable order) and how many rows to return.
// MountRead decodes it and applies Cap(args.Limit) automatically.
//
// Example: var args crud.ReadArgs; if err := ctx.Decode(&args); err != nil { … }
type ReadArgs struct {
	ID     string
	Search string
	After  string
	Limit  int64
}

// ReadArgsModel is the wire schema of ReadArgs.
var ReadArgsModel = model.Definition{
	Name: "read_args",
	Wire: true,
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), Permitted: model.Permitted{Maximum: MaxID}},
		{Name: "search", Type: model.Text(), Permitted: model.Permitted{Maximum: MaxSearch}},
		{Name: "after", Type: model.Text(), Permitted: model.Permitted{Maximum: MaxAfter}},
		{Name: "limit", Type: model.Int()},
	},
}

func (a *ReadArgs) Schema() []model.Field { return ReadArgsModel.Fields }
func (a *ReadArgs) Pointers() []any       { return []any{&a.ID, &a.Search, &a.After, &a.Limit} }
func (a *ReadArgs) IsNil() bool           { return a == nil }

func (a *ReadArgs) EncodeFields(w model.FieldWriter) {
	if a.ID != "" {
		w.String("id", a.ID)
	}
	w.String("search", a.Search)
	w.String("after", a.After)
	w.Int("limit", a.Limit)
}

func (a *ReadArgs) DecodeFields(r model.FieldReader) {
	a.ID, _ = r.String("id")
	a.Search, _ = r.String("search")
	a.After, _ = r.String("after")
	a.Limit, _ = r.Int("limit")
}

// MountRead registers Op(m, model.Read) as the read operation of an entity:
// it decodes ReadArgs, caps args.Limit to MaxPage via Cap, sanitizes Search, After and ID,
// calls read(args) and encodes the returned ModelSlice.
//
//	crud.MountRead(reg, &Device{}, func(args crud.ReadArgs) (model.ModelSlice, error) {
//		return m.ReadDevices(args.ID, args.Search, args.After, args.Limit)
//	})
func MountRead(reg router.OperationRegistry, m model.ModuleNaming, read func(args ReadArgs) (model.ModelSlice, error)) router.Route {
	return mountOp(reg, m, model.Read, &ReadArgs{}, func(ctx router.Context) {
		var args ReadArgs
		if err := ctx.Decode(&args); err != nil {
			rpc.WriteError(ctx, ErrCannotRead)
			return
		}
		args.Limit = Cap(args.Limit)
		if len(args.Search) > MaxSearch {
			args.Search = args.Search[:MaxSearch]
		}
		if len(args.After) > MaxAfter {
			args.After = args.After[:MaxAfter]
		}
		if len(args.ID) > MaxID {
			args.ID = args.ID[:MaxID]
		}
		list, err := read(args)
		if err != nil {
			rpc.WriteError(ctx, err)
			return
		}
		if enc, ok := list.(model.Encodable); ok {
			if err := ctx.Encode(enc); err != nil {
				rpc.WriteError(ctx, err)
			}
		}
	})
}
