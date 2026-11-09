package crud

import (
	"webtyp.com/model"
	"webtyp.com/router"
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
// crud applies it automatically to ReadArgs.Limit.
func Cap(limit int64) int64 {
	if limit <= 0 || limit > MaxPage {
		return MaxPage
	}
	return limit
}

// ReadArgs is what a read operation receives: an optional record ID for single-row lookup,
// the search term ("" = no search), the id of the last row already shown ("" = from the start;
// the server returns the rows after it in its own stable order) and how many rows to return.
// Registry.Read decodes it and applies Cap(args.Limit) automatically.
//
// Example: var args crud.ReadArgs; if err := ctx.Decode(&args); err != nil { … }
type ReadArgs struct {
	ID     string
	Search string
	After  string
	Limit  int64
	// Filter is the entity's typed filter, nil when the entity declares none; decoded from the key "filter".
	Filter model.Model
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
	if a.Search != "" {
		w.String("search", a.Search)
	}
	if a.After != "" {
		w.String("after", a.After)
	}
	if a.Limit != 0 {
		w.Int("limit", a.Limit)
	}
	if a.Filter != nil {
		w.Object(keyFilter, a.Filter)
	}
}

func (a *ReadArgs) DecodeFields(r model.FieldReader) {
	a.ID, _ = r.String("id")
	a.Search, _ = r.String("search")
	a.After, _ = r.String("after")
	a.Limit, _ = r.Int("limit")
	if a.Filter != nil {
		r.Object(keyFilter, a.Filter)
	}
}

func (r *operationMounter) AddReadOnlyEntity(m model.ModuleNaming, newFilter func() model.Model, read func(c CurrentUser, args ReadArgs, filter model.Model) (model.ModelSlice, error)) Route {
	if m == nil || read == nil {
		return r.failMount(ErrNilFunction)
	}
	if m.ModelName() == "" {
		return r.failMount(ErrEmptyModelName)
	}
	r.mountedEntities = append(r.mountedEntities, m.ModelName())
	h := func(ctx router.Context) {
		args := ReadArgs{}
		if newFilter != nil {
			args.Filter = newFilter()
		}
		if err := ctx.Decode(&args); err != nil {
			router.WriteError(ctx, ErrCannotRead)
			return
		}
		if err := validateReadArgs(&args); err != nil {
			router.WriteError(ctx, err)
			return
		}
		list, err := read(currentUserOf(ctx), args, args.Filter)
		if err != nil {
			router.WriteError(ctx, err)
			return
		}
		if enc, ok := list.(model.Encodable); ok {
			if err := ctx.Encode(enc); err != nil {
				router.WriteError(ctx, err)
			}
		}
	}
	if r.reg == nil {
		return mountedRoute{}
	}
	rt := r.reg.Operation(router.Operation{Module: r.module, Entity: m.ModelName(), Action: model.Read}, h)
	if r.isPublicAction(m.ModelName(), model.Read) {
		rt.Public()
	} else {
		rt.Requires(model.Resource(m.ModelName()), model.Read)
	}
	rt.Accepts(&ReadArgs{})
	return mountedRoute{rt}
}
