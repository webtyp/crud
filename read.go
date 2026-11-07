package crud

import "webtyp.com/model"

// MaxPage is the most rows one read call returns; the server caps every request to it (Cap).
const MaxPage = 500

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
		{Name: "id", Type: model.Text()},
		{Name: "search", Type: model.Text()},
		{Name: "after", Type: model.Text()},
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
