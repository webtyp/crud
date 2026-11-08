package tests

import (
	"errors"
	"strings"
	"testing"
	"webtyp.com/crud"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/storage"
)

type tRec struct {
	ID        string
	Name      string
	Note      string
	CreatedAt int64
	UpdatedAt int64
}
func (m *tRec) ModelName() string { return "t" }
func (m *tRec) Allow() model.Action { return model.AllActions }
func (m *tRec) IsNil() bool { return m == nil }
func (m *tRec) EncodeFields(w model.FieldWriter) {
	w.String("id", m.ID)
	w.String("name", m.Name)
	w.String("note", m.Note)
	w.Int("created_at", m.CreatedAt)
	w.Int("updated_at", m.UpdatedAt)
}
func (m *tRec) DecodeFields(r model.FieldReader) {
	if v, ok := r.String("id"); ok { m.ID = v }
	if v, ok := r.String("name"); ok { m.Name = v }
	if v, ok := r.String("note"); ok { m.Note = v }
	if v, ok := r.Int("created_at"); ok { m.CreatedAt = v }
	if v, ok := r.Int("updated_at"); ok { m.UpdatedAt = v }
}
func (m *tRec) Pointers() []any {
	return []any{&m.ID, &m.Name, &m.Note, &m.CreatedAt, &m.UpdatedAt}
}
func (m *tRec) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true, AutoInc: true}},
		{Name: "name", Type: model.Text(), Item: model.ItemLabel},
		{Name: "note", Type: model.Text()},
		{Name: "created_at", Type: model.Int(), ServerOnly: true},
		{Name: "updated_at", Type: model.Int(), ServerOnly: true},
	}
}

type tRecList []tRec
func (l *tRecList) Append() model.Fielder { *l = append(*l, tRec{}); return &(*l)[len(*l)-1] }
func (l *tRecList) Len() int { return len(*l) }
func (l *tRecList) At(i int) model.Fielder { return &(*l)[i] }
func (l *tRecList) EncodeFields(w model.FieldWriter) {
	a := w.Array("records", len(*l))
	for i := range *l {
		a.Object(&(*l)[i])
	}
}
func (l *tRecList) DecodeFields(r model.FieldReader) {
	if a, ok := r.Array("records"); ok {
		for i := 0; i < a.Len(); i++ {
			f := l.Append().(model.Decodable)
			a.Object(i, f)
		}
	}
}
func (l *tRecList) IsNil() bool { return l == nil }

type tMod struct {
	table crud.Table
}
func (m *tMod) ModelName() string { return "tmod" }
func (m *tMod) MountOperations(reg crud.Registry) {
	reg.Table(m.table)
}

type noAllow struct { tRec }
func (m *noAllow) Allow() model.Action { return 0 }

func TestTable_NothingAllowedFailsMount(t *testing.T) {

	err := crud.Mount(nil, nil, &tMod{table: crud.Table{
		Model: &noAllow{},
		NewList: func() model.ModelSlice { return &tRecList{} },
	}})
	if err == nil || err.Error() != string(crud.ErrNothingAllowed) {
		t.Errorf("expected ErrNothingAllowed, got %v", err)
	}
}

type justRead struct { tRec }
func (m *justRead) Allow() model.Action { return model.Create | model.Read }
func TestTable_NotAllowedActionDoesNotExist(t *testing.T) {
	reg := &mockRouter{}
	crud.Mount(reg, nil, &tMod{table: crud.Table{
		Model: &justRead{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store: newMemStore(),
	}})
	if reg.Has("t_u") {
		t.Errorf("update should not be mounted")
	}
}

type routeStub struct{}
func (routeStub) RequiresMFA() router.Route { return routeStub{} }
func (routeStub) Describe(string) router.Route { return routeStub{} }
func (routeStub) Accepts(args model.Fielder) router.Route { return routeStub{} }
func (routeStub) Public() router.Route { return routeStub{} }
func (routeStub) Authenticated() router.Route { return routeStub{} }
func (routeStub) Requires(r model.Resource, a model.Action) router.Route { return routeStub{} }


type mockRouter struct{
	ops map[string]bool
}
func (r *mockRouter) Operation(op string, h router.HandlerFunc) router.Route {
	if r.ops == nil { r.ops = make(map[string]bool) }
	r.ops[op] = true
	return routeStub{}
}
func (r *mockRouter) Has(op string) bool {
	return r.ops[op]
}

func TestTable_HookWithoutActionFailsMount(t *testing.T) {

err := crud.Mount(nil, nil, &tMod{table: crud.Table{
		Model: &justRead{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store: newMemStore(),
		Update: func(c crud.Caller, tx crud.Tx, old, next model.Model, fields []string) error { return nil },
	}})
	if err == nil || err.Error() != string(crud.ErrHookWithoutAction) {
		t.Errorf("expected ErrHookWithoutAction, got %v", err)
	}
}

func TestTable_CreateSetsIDAndStamps(t *testing.T) {
	mod := &tMod{table: crud.Table{
		Model: &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store: newMemStore(),
	}}
	caller := crud.LoopbackActingAs("u1", mod)
	var saved tRec
	var err error
	caller.Call("tmod."+crud.Op(&tRec{}, model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{}}}, &saved, func(e error){ err = e })
	if err != nil { t.Fatal(err) }
	if saved.ID == "" { t.Errorf("expected id") }
	if saved.CreatedAt == 0 || saved.UpdatedAt != saved.CreatedAt { t.Errorf("expected stamps %d %d", saved.CreatedAt, saved.UpdatedAt) }
}

func TestTable_BatchOnNonTransactionalStoreRejected(t *testing.T) {
	mod := &tMod{table: crud.Table{
		Model: &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store: newMemStore(), // Transactional implemented
	}}
	type nonTxStore struct { crud.Store }
	mod.table.Store = &nonTxStore{newMemStore()}
	caller := crud.LoopbackActingAs("u1", mod)

	var err error
	caller.Call("tmod."+crud.Op(&tRec{}, model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{}, &tRec{}}}, nil, func(e error){ err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrBatchNotAtomic) {
		t.Errorf("expected ErrBatchNotAtomic, got %v", err)
	}

	caller.Call("tmod."+crud.Op(&tRec{}, model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{}}}, nil, func(e error){ err = e })
	if err != nil { t.Errorf("1 record should be allowed: %v", err) }
}

func TestTable_CreateBatchIsAtomic(t *testing.T) {
	store := newMemStore()
	mod := &tMod{table: crud.Table{
		Model: &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store: store,
		Create: func(c crud.Caller, tx crud.Tx, rec model.Model) error {
			if rec.(*tRec).Name == "fail" { return model.Conflict("hook fail") }
			return nil
		},
	}}
	caller := crud.LoopbackActingAs("u1", mod)
	var err error
	caller.Call("tmod."+crud.Op(&tRec{}, model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{Name: "ok"}, &tRec{Name: "fail"}}}, nil, func(e error){ err = e })
	if err == nil || err.Error() != "loopback: 409 hook fail" { t.Errorf("expected hook fail, got %v", err) }

	if len(store.rows) != 0 { t.Errorf("expected rollback") }
}

func TestTable_UpdateOnlyListedFields(t *testing.T) {
	store := newMemStore()
	store.Create(&tRec{ID: "r1", Name: "a", Note: "x"})
	mod := &tMod{table: crud.Table{
		Model: &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store: store,
	}}
	caller := crud.LoopbackActingAs("u1", mod)
	var err error
	caller.Call("tmod."+crud.Op(&tRec{}, model.Update), &crud.UpdateArgs{IDs: []string{"r1"}, Fields: []string{"name"}, Record: &tRec{Name: "b", Note: "y"}}, nil, func(e error){ err = e })
	if err != nil { t.Fatal(err) }

	var rec tRec
	store.ReadOne(&rec, "r1")
	if rec.Name != "b" || rec.Note != "x" {
		t.Errorf("expected Name b and Note x, got %s %s", rec.Name, rec.Note)
	}
}

func TestTable_UpdateMissingIsNotFound(t *testing.T) {
	mod := &tMod{table: crud.Table{
		Model: &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store: newMemStore(),
	}}
	caller := crud.LoopbackActingAs("u1", mod)
	var err error
	caller.Call("tmod."+crud.Op(&tRec{}, model.Update), &crud.UpdateArgs{IDs: []string{"nope"}, Fields: []string{"name"}, Record: &tRec{}}, nil, func(e error){ err = e })
	if err == nil || err.Error() != "loopback: 404 not found" { t.Errorf("expected 404, got %v", err) }
}

func TestTable_UpdateHookSeesOldAndNext(t *testing.T) {
	store := newMemStore()
	store.Create(&tRec{ID: "r1", Name: "locked", Note: "x"})
	mod := &tMod{table: crud.Table{
		Model: &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store: store,
		Update: func(c crud.Caller, tx crud.Tx, old, next model.Model, fields []string) error {
			if old.(*tRec).Name == "locked" { return model.Conflict("reject") }
			return nil
		},
	}}
	caller := crud.LoopbackActingAs("u1", mod)
	var err error
	caller.Call("tmod."+crud.Op(&tRec{}, model.Update), &crud.UpdateArgs{IDs: []string{"r1"}, Fields: []string{"name"}, Record: &tRec{Name: "b"}}, nil, func(e error){ err = e })
	if err == nil || !strings.Contains(err.Error(), "reject") { t.Errorf("expected reject, got %v", err) }
}

func TestTable_CommittedAfterCommitOnly(t *testing.T) {
	store := newMemStore()
	called := 0
	mod := &tMod{table: crud.Table{
		Model: &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store: store,
		Create: func(c crud.Caller, tx crud.Tx, rec model.Model) error { return errors.New("fail") },
		Committed: func(c crud.Caller, a model.Action, rec model.Model, fields []string) { called++ },
	}}
	caller := crud.LoopbackActingAs("u1", mod)
	caller.Call("tmod."+crud.Op(&tRec{}, model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{}}}, nil, func(e error){})
	if called > 0 { t.Errorf("committed called on fail") }
}

func TestTable_CommittedUpdateCarriesEachID(t *testing.T) {
	store := newMemStore()
	store.Create(&tRec{ID: "a"})
	store.Create(&tRec{ID: "b"})
	var ids []string
	mod := &tMod{table: crud.Table{
		Model: &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store: store,
		Committed: func(c crud.Caller, a model.Action, rec model.Model, fields []string) { ids = append(ids, rec.(*tRec).ID) },
	}}
	caller := crud.LoopbackActingAs("u1", mod)
	caller.Call("tmod."+crud.Op(&tRec{}, model.Update), &crud.UpdateArgs{IDs: []string{"a", "b"}, Fields: []string{"name"}, Record: &tRec{}}, nil, func(e error){})
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "b" { t.Errorf("expected a, b got %v", ids) }
}

func TestTable_SearchDefaultUsesItemLabel(t *testing.T) {
	store := newMemStore()
	store.Create(&tRec{ID: "1", Name: "Ana"})
	store.Create(&tRec{ID: "2", Name: "Bruno"})
	mod := &tMod{table: crud.Table{
		Model: &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store: store,
	}}
	caller := crud.LoopbackActingAs("u1", mod)
	var list tRecList
	var err error
	caller.Call("tmod."+crud.Op(&tRec{}, model.Read), &crud.ReadArgs{Search: "an"}, &list, func(e error){ err = e })
	if err != nil { t.Fatal(err) }
	if len(list) != 1 || list[0].Name != "Ana" { t.Errorf("expected Ana, got %v", list) }
}

func TestTable_SearchHookOrIsRejected(t *testing.T) {
	mod := &tMod{table: crud.Table{
		Model: &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store: newMemStore(),
		Search: func(c crud.Caller, text string) ([]storage.Condition, error) {
			return []storage.Condition{storage.Or(storage.Eq("name", "1"))}, nil
		},
	}}
	caller := crud.LoopbackActingAs("u1", mod)
	var err error
	caller.Call("tmod."+crud.Op(&tRec{}, model.Read), &crud.ReadArgs{Search: "an"}, nil, func(e error){ err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrOrCondition) { t.Errorf("expected ErrOrCondition, got %v", err) }
}

type cpKeyRec struct { RoomID string; EqID string }
func (m *cpKeyRec) ModelName() string { return "cp" }
func (m *cpKeyRec) Allow() model.Action { return model.AllActions }
func (m *cpKeyRec) IsNil() bool { return m == nil }
func (m *cpKeyRec) EncodeFields(w model.FieldWriter) { w.String("room_id", m.RoomID); w.String("eq_id", m.EqID) }
func (m *cpKeyRec) DecodeFields(r model.FieldReader) {
	if v, ok := r.String("room_id"); ok { m.RoomID = v }
	if v, ok := r.String("eq_id"); ok { m.EqID = v }
}
func (m *cpKeyRec) Pointers() []any { return []any{&m.RoomID, &m.EqID} }
func (m *cpKeyRec) Schema() []model.Field { return []model.Field{
	{Name: "room_id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
	{Name: "eq_id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
}}

type cpKeyList []cpKeyRec
func (l *cpKeyList) Append() model.Fielder { *l = append(*l, cpKeyRec{}); return &(*l)[len(*l)-1] }
func (l *cpKeyList) Len() int { return len(*l) }
func (l *cpKeyList) At(i int) model.Fielder { return &(*l)[i] }
func (l *cpKeyList) EncodeFields(w model.FieldWriter) {
	a := w.Array("records", len(*l))
	for i := range *l { a.Object(&(*l)[i]) }
}
func (l *cpKeyList) DecodeFields(r model.FieldReader) {
	if a, ok := r.Array("records"); ok {
		for i := 0; i < a.Len(); i++ { a.Object(i, l.Append().(model.Decodable)) }
	}
}
func (l *cpKeyList) IsNil() bool { return l == nil }

func TestTable_CompositeKeyRoundTrip(t *testing.T) {
	store := newMemStore()
	mod := &tMod{table: crud.Table{
		Model: &cpKeyRec{},
		NewList: func() model.ModelSlice { return &cpKeyList{} },
		Store: store,
	}}
	caller := crud.LoopbackActingAs("u1", mod)
	caller.Call("tmod."+crud.Op(&cpKeyRec{}, model.Create), &crud.CreateArgs{Records: []model.Model{&cpKeyRec{RoomID: "r1", EqID: "e7"}}}, nil, func(e error){})

	var list cpKeyList
	var err error
	caller.Call("tmod."+crud.Op(&cpKeyRec{}, model.Read), &crud.ReadArgs{ID: "r1/e7"}, &list, func(e error){ err = e })
	if err != nil { t.Fatal(err) }
	if len(list) != 1 || list[0].RoomID != "r1" { t.Errorf("expected r1, got %v", list) }

	caller.Call("tmod."+crud.Op(&cpKeyRec{}, model.Delete), &crud.DeleteArgs{IDs: []string{"r1/e7"}}, nil, func(e error){})
	caller.Call("tmod."+crud.Op(&cpKeyRec{}, model.Read), &crud.ReadArgs{ID: "r1/e7"}, nil, func(e error){ err = e })
	if err == nil { t.Errorf("expected not found") }
}

func TestTable_CompositeKeyPartRequired(t *testing.T) {
	mod := &tMod{table: crud.Table{
		Model: &cpKeyRec{},
		NewList: func() model.ModelSlice { return &cpKeyList{} },
		Store: newMemStore(),
	}}
	caller := crud.LoopbackActingAs("u1", mod)
	var err error
	caller.Call("tmod."+crud.Op(&cpKeyRec{}, model.Create), &crud.CreateArgs{Records: []model.Model{&cpKeyRec{RoomID: "r1"}}}, nil, func(e error){ err = e })
	if err == nil || !strings.Contains(err.Error(), "required") { t.Errorf("expected required, got %v", err) }
}

func TestTable_KeyWithSeparatorRejected(t *testing.T) {
	mod := &tMod{table: crud.Table{
		Model: &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store: newMemStore(),
	}}
	caller := crud.LoopbackActingAs("u1", mod)
	var err error
	caller.Call("tmod."+crud.Op(&tRec{}, model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r/1"}}}, nil, func(e error){ err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrKeyHasSeparator) { t.Errorf("expected ErrKeyHasSeparator, got %v", err) }
}

type intKeyRec struct { ShiftID string; SpecificDate int64 }
func (m *intKeyRec) ModelName() string { return "intkey" }
func (m *intKeyRec) Allow() model.Action { return model.AllActions }
func (m *intKeyRec) IsNil() bool { return m == nil }
func (m *intKeyRec) EncodeFields(w model.FieldWriter) { w.String("shift_id", m.ShiftID); w.Int("specific_date", m.SpecificDate) }
func (m *intKeyRec) DecodeFields(r model.FieldReader) {
	if v, ok := r.String("shift_id"); ok { m.ShiftID = v }
	if v, ok := r.Int("specific_date"); ok { m.SpecificDate = v }
}
func (m *intKeyRec) Pointers() []any { return []any{&m.ShiftID, &m.SpecificDate} }
func (m *intKeyRec) Schema() []model.Field { return []model.Field{
	{Name: "shift_id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
	{Name: "specific_date", Type: model.Int(), DB: &model.FieldDB{PK: true}},
}}

type intKeyList []intKeyRec
func (l *intKeyList) Append() model.Fielder { *l = append(*l, intKeyRec{}); return &(*l)[len(*l)-1] }
func (l *intKeyList) Len() int { return len(*l) }
func (l *intKeyList) At(i int) model.Fielder { return &(*l)[i] }
func (l *intKeyList) EncodeFields(w model.FieldWriter) {
	a := w.Array("records", len(*l))
	for i := range *l { a.Object(&(*l)[i]) }
}
func (l *intKeyList) DecodeFields(r model.FieldReader) {
	if a, ok := r.Array("records"); ok {
		for i := 0; i < a.Len(); i++ { a.Object(i, l.Append().(model.Decodable)) }
	}
}
func (l *intKeyList) IsNil() bool { return l == nil }

func TestTable_CompositeKeyWithInt(t *testing.T) {
	mod := &tMod{table: crud.Table{
		Model: &intKeyRec{},
		NewList: func() model.ModelSlice { return &intKeyList{} },
		Store: newMemStore(),
	}}
	caller := crud.LoopbackActingAs("u1", mod)
	caller.Call("tmod."+crud.Op(&intKeyRec{}, model.Create), &crud.CreateArgs{Records: []model.Model{&intKeyRec{ShiftID: "s1", SpecificDate: 1700000000}}}, nil, func(e error){})
	var list intKeyList
	var err error
	caller.Call("tmod."+crud.Op(&intKeyRec{}, model.Read), &crud.ReadArgs{ID: "s1/1700000000"}, &list, func(e error){ err = e })
	if err != nil { t.Fatal(err) }
	if len(list) != 1 || list[0].ShiftID != "s1" || list[0].SpecificDate != 1700000000 { t.Errorf("expected read, got %v", list) }
}

func TestTable_PublicCreateWithoutSession(t *testing.T) {
	reg := &mockRouter{}
	crud.Mount(reg, []crud.Public{{Model: &tRec{}, Actions: model.Create}}, &tMod{table: crud.Table{
		Model: &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store: newMemStore(),
		Create: func(c crud.Caller, tx crud.Tx, rec model.Model) error {
			return nil
		},
	}})
	if !reg.Has("t_c") { t.Errorf("expected t_c mounted") }
}

func TestTable_ServerOnlyRejected(t *testing.T) {
	mod := &tMod{table: crud.Table{
		Model: &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store: newMemStore(),
	}}
	caller := crud.LoopbackActingAs("u1", mod)
	var err error
	caller.Call("tmod."+crud.Op(&tRec{}, model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{CreatedAt: 123}}}, nil, func(e error){ err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrServerOnlyField) {
		t.Errorf("expected ErrServerOnlyField, got %v", err)
	}
}


type filterRec struct { Status string }
func (f *filterRec) ModelName() string { return "filter" }
func (f *filterRec) Allow() model.Action { return model.AllActions }
func (f *filterRec) Schema() []model.Field { return []model.Field{{Name: "status", Type: model.Text()}} }
func (f *filterRec) Pointers() []any { return []any{&f.Status} }
func (f *filterRec) DecodeFields(r model.FieldReader) {
	if v, ok := r.String("status"); ok { f.Status = v }
}
func (f *filterRec) EncodeFields(w model.FieldWriter) {
	w.String("status", f.Status)
}
func (f *filterRec) IsNil() bool { return f == nil }

func TestTable_FilterEquality(t *testing.T) {
	store := newMemStore()
	store.Create(&tRec{ID: "1", Name: "open"})
	store.Create(&tRec{ID: "2", Name: "closed"})
	mod := &tMod{table: crud.Table{
		Model: &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store: store,
		NewFilter: func() model.Model { return &filterRec{} },
	}}
	caller := crud.LoopbackActingAs("u1", mod)
	var list tRecList
	caller.Call("tmod."+crud.Op(&tRec{}, model.Read), &crud.ReadArgs{Filter: &filterRec{Status: "open"}}, &list, func(e error){})
}
