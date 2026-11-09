package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/fmt"
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

func (m *tRec) ModelName() string        { return "t" }
func (m *tRec) Allow() model.Action      { return model.AllActions }
func (m *tRec) IsNil() bool              { return m == nil }
func (m *tRec) EncodeFields(w model.FieldWriter) {
	w.String("id", m.ID)
	w.String("name", m.Name)
	w.String("note", m.Note)
	w.Int("created_at", m.CreatedAt)
	w.Int("updated_at", m.UpdatedAt)
}
func (m *tRec) DecodeFields(r model.FieldReader) {
	if v, ok := r.String("id"); ok {
		m.ID = v
	}
	if v, ok := r.String("name"); ok {
		m.Name = v
	}
	if v, ok := r.String("note"); ok {
		m.Note = v
	}
	if v, ok := r.Int("created_at"); ok {
		m.CreatedAt = v
	}
	if v, ok := r.Int("updated_at"); ok {
		m.UpdatedAt = v
	}
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
func (l *tRecList) Len() int             { return len(*l) }
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

type testModule struct {
	table crud.StoredEntity
}

func (m *testModule) ModelName() string { return "tmod" }
func (m *testModule) MountOperations(reg crud.Registry) {
	reg.AddStoredEntity(m.table)
}
func (m *testModule) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{Name: "tmod", Entities: []model.Definition{{
		Name:  m.table.Model.ModelName(),
		Allow: m.table.Model.(model.Allower).Allow(),
		Rules: m.table.Rules,
	}}}
}

type noAllow struct{ tRec }

func (m *noAllow) Allow() model.Action { return 0 }

func TestStoredEntity_NothingAllowedFailsMount(t *testing.T) {
	err := crud.Mount(nil, nil, nil, &testModule{table: crud.StoredEntity{
		Model:   &noAllow{},
		NewList: func() model.ModelSlice { return &tRecList{} },
	}})
	if err == nil || err.Error() != string(crud.ErrNothingAllowed) {
		t.Errorf("expected ErrNothingAllowed, got %v", err)
	}
}

type justRead struct{ tRec }

func (m *justRead) Allow() model.Action { return model.Create | model.Read }

func TestStoredEntity_NotAllowedActionDoesNotExist(t *testing.T) {
	reg := &mockRouter{}
	crud.Mount(reg, nil, nil, &testModule{table: crud.StoredEntity{
		Model:   &justRead{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.Create|model.Read, 0),
	}})
	if reg.Has("t_update") {
		t.Errorf("update should not be mounted")
	}
}

type routeStub struct{}

func (routeStub) RequiresMFA() router.Route                             { return routeStub{} }
func (routeStub) Describe(string) router.Route                          { return routeStub{} }
func (routeStub) Accepts(args model.Fielder) router.Route               { return routeStub{} }
func (routeStub) Public() router.Route                                  { return routeStub{} }
func (routeStub) Authenticated() router.Route                           { return routeStub{} }
func (routeStub) Requires(r model.Resource, a model.Action) router.Route { return routeStub{} }

type mockRouter struct {
	ops map[string]bool
}

func (r *mockRouter) Operation(op router.Operation, h router.HandlerFunc) router.Route {
	if r.ops == nil {
		r.ops = make(map[string]bool)
	}
	r.ops[op.Entity+"_"+op.Action.Verb()] = true
	return routeStub{}
}

func (r *mockRouter) Has(op string) bool {
	return r.ops[op]
}

func TestStoredEntity_HookWithoutActionFailsMount(t *testing.T) {
	err := crud.Mount(nil, nil, nil, &testModule{table: crud.StoredEntity{
		Model:   &justRead{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.Create|model.Read, 0),
		Update: func(c crud.CurrentUser, tx crud.Tx, old, next model.Model, fields []string) (crud.RuleAnswer, error) {
			return crud.ApplyAndPublish, nil
		},
	}})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestStoredEntity_CreateSetsIDAndStamps(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)
	var saved tRec
	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{}}}, &saved, func(e error) { err = e })
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" {
		t.Errorf("expected id")
	}
	if saved.CreatedAt == 0 || saved.UpdatedAt != saved.CreatedAt {
		t.Errorf("expected stamps %d %d", saved.CreatedAt, saved.UpdatedAt)
	}
}

func TestStoredEntity_BatchOnNonTransactionalStoreRejected(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, 0),
	}}
	type nonTxStore struct{ crud.Store }
	mod.table.Store = &nonTxStore{newMemStore()}
	caller := crud.LoopbackActingAs(nil, "u1", mod)

	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{}, &tRec{}}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrBatchNotAtomic) {
		t.Errorf("expected ErrBatchNotAtomic, got %v", err)
	}

	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{}}}, nil, func(e error) { err = e })
	if err != nil {
		t.Errorf("1 record should be allowed: %v", err)
	}
}

func TestStoredEntity_CreateBatchIsAtomic(t *testing.T) {
	store := newMemStore()
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, model.Create),
		Create: func(c crud.CurrentUser, tx crud.Tx, rec model.Model) (crud.RuleAnswer, error) {
			if rec.(*tRec).Name == "fail" {
				return 0, model.Conflict("hook fail")
			}
			return crud.ApplyAndPublish, nil
		},
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)
	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{Name: "ok"}, &tRec{Name: "fail"}}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 409 hook fail" {
		t.Errorf("expected hook fail, got %v", err)
	}

	if len(store.rows) != 0 {
		t.Errorf("expected rollback")
	}
}

func TestStoredEntity_UpdateOnlyListedFields(t *testing.T) {
	store := newMemStore()
	store.Create(&tRec{ID: "r1", Name: "a", Note: "x"})
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)
	var err error
	caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: []string{"r1"}, Fields: []string{"name"}, Record: &tRec{Name: "b", Note: "y"}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatal(err)
	}

	var rec tRec
	store.ReadOne(&rec, "r1")
	if rec.Name != "b" || rec.Note != "x" {
		t.Errorf("expected Name b and Note x, got %s %s", rec.Name, rec.Note)
	}
}

func TestStoredEntity_UpdateMissingIsNotFound(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)
	var err error
	caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: []string{"nope"}, Fields: []string{"name"}, Record: &tRec{}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 404 not found" {
		t.Errorf("expected 404, got %v", err)
	}
}

func TestStoredEntity_UpdateHookSeesOldAndNext(t *testing.T) {
	store := newMemStore()
	store.Create(&tRec{ID: "r1", Name: "locked", Note: "x"})
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, model.Update),
		Update: func(c crud.CurrentUser, tx crud.Tx, old, next model.Model, fields []string) (crud.RuleAnswer, error) {
			if old.(*tRec).Name == "locked" {
				return 0, model.Conflict("reject")
			}
			return crud.ApplyAndPublish, nil
		},
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)
	var err error
	caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: []string{"r1"}, Fields: []string{"name"}, Record: &tRec{Name: "b"}}, nil, func(e error) { err = e })
	if err == nil || !fmt.Contains(err.Error(), "reject") {
		t.Errorf("expected reject, got %v", err)
	}
}

func TestStoredEntity_SearchDefaultUsesItemLabel(t *testing.T) {
	store := newMemStore()
	store.Create(&tRec{ID: "1", Name: "Ana"})
	store.Create(&tRec{ID: "2", Name: "Bruno"})
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)
	var list tRecList
	var err error
	caller.Call(operationOf(model.Read), &crud.ReadArgs{Search: "an"}, &list, func(e error) { err = e })
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "Ana" {
		t.Errorf("expected Ana, got %v", list)
	}
}

func TestStoredEntity_SearchHookOrIsRejected(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, model.Read),
		Search: func(c crud.CurrentUser, text string) ([]storage.Condition, error) {
			return []storage.Condition{storage.Or(storage.Eq("name", "1"))}, nil
		},
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)
	var err error
	caller.Call(operationOf(model.Read), &crud.ReadArgs{Search: "an"}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrOrCondition) {
		t.Errorf("expected ErrOrCondition, got %v", err)
	}
}

type cpKeyRec struct {
	RoomID string
	EqID   string
}

func (m *cpKeyRec) ModelName() string        { return "cp" }
func (m *cpKeyRec) Allow() model.Action      { return model.AllActions }
func (m *cpKeyRec) IsNil() bool              { return m == nil }
func (m *cpKeyRec) EncodeFields(w model.FieldWriter) {
	w.String("room_id", m.RoomID)
	w.String("eq_id", m.EqID)
}
func (m *cpKeyRec) DecodeFields(r model.FieldReader) {
	if v, ok := r.String("room_id"); ok {
		m.RoomID = v
	}
	if v, ok := r.String("eq_id"); ok {
		m.EqID = v
	}
}
func (m *cpKeyRec) Pointers() []any { return []any{&m.RoomID, &m.EqID} }
func (m *cpKeyRec) Schema() []model.Field {
	return []model.Field{
		{Name: "room_id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "eq_id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
	}
}

type cpKeyList []cpKeyRec

func (l *cpKeyList) Append() model.Fielder { *l = append(*l, cpKeyRec{}); return &(*l)[len(*l)-1] }
func (l *cpKeyList) Len() int             { return len(*l) }
func (l *cpKeyList) At(i int) model.Fielder { return &(*l)[i] }
func (l *cpKeyList) EncodeFields(w model.FieldWriter) {
	a := w.Array("records", len(*l))
	for i := range *l {
		a.Object(&(*l)[i])
	}
}
func (l *cpKeyList) DecodeFields(r model.FieldReader) {
	if a, ok := r.Array("records"); ok {
		for i := 0; i < a.Len(); i++ {
			a.Object(i, l.Append().(model.Decodable))
		}
	}
}
func (l *cpKeyList) IsNil() bool { return l == nil }

func TestStoredEntity_CompositeKeyRoundTrip(t *testing.T) {
	store := newMemStore()
	mod := &testModule{table: crud.StoredEntity{
		Model:   &cpKeyRec{},
		NewList: func() model.ModelSlice { return &cpKeyList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)
	caller.Call(router.Operation{Module: "tmod", Entity: "cp", Action: model.Create}, &crud.CreateArgs{Records: []model.Model{&cpKeyRec{RoomID: "r1", EqID: "e7"}}}, nil, func(e error) {})

	var list cpKeyList
	var err error
	caller.Call(router.Operation{Module: "tmod", Entity: "cp", Action: model.Read}, &crud.ReadArgs{ID: "r1/e7"}, &list, func(e error) { err = e })
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].RoomID != "r1" {
		t.Errorf("expected r1, got %v", list)
	}

	caller.Call(router.Operation{Module: "tmod", Entity: "cp", Action: model.Delete}, &crud.DeleteArgs{IDs: []string{"r1/e7"}}, nil, func(e error) {})
	caller.Call(router.Operation{Module: "tmod", Entity: "cp", Action: model.Read}, &crud.ReadArgs{ID: "r1/e7"}, nil, func(e error) { err = e })
	if err == nil {
		t.Errorf("expected not found")
	}
}

func TestStoredEntity_CompositeKeyPartRequired(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &cpKeyRec{},
		NewList: func() model.ModelSlice { return &cpKeyList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)
	var err error
	caller.Call(router.Operation{Module: "tmod", Entity: "cp", Action: model.Create}, &crud.CreateArgs{Records: []model.Model{&cpKeyRec{RoomID: "r1"}}}, nil, func(e error) { err = e })
	if err == nil || !fmt.Contains(err.Error(), "required") {
		t.Errorf("expected required, got %v", err)
	}
}

func TestStoredEntity_KeyWithSeparatorRejected(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)
	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r/1"}}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrKeyHasSeparator) {
		t.Errorf("expected ErrKeyHasSeparator, got %v", err)
	}
}

type intKeyRec struct {
	ShiftID      string
	SpecificDate int64
}

func (m *intKeyRec) ModelName() string        { return "intkey" }
func (m *intKeyRec) Allow() model.Action      { return model.AllActions }
func (m *intKeyRec) IsNil() bool              { return m == nil }
func (m *intKeyRec) EncodeFields(w model.FieldWriter) {
	w.String("shift_id", m.ShiftID)
	w.Int("specific_date", m.SpecificDate)
}
func (m *intKeyRec) DecodeFields(r model.FieldReader) {
	if v, ok := r.String("shift_id"); ok {
		m.ShiftID = v
	}
	if v, ok := r.Int("specific_date"); ok {
		m.SpecificDate = v
	}
}
func (m *intKeyRec) Pointers() []any { return []any{&m.ShiftID, &m.SpecificDate} }
func (m *intKeyRec) Schema() []model.Field {
	return []model.Field{
		{Name: "shift_id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "specific_date", Type: model.Int(), DB: &model.FieldDB{PK: true}},
	}
}

type intKeyList []intKeyRec

func (l *intKeyList) Append() model.Fielder { *l = append(*l, intKeyRec{}); return &(*l)[len(*l)-1] }
func (l *intKeyList) Len() int             { return len(*l) }
func (l *intKeyList) At(i int) model.Fielder { return &(*l)[i] }
func (l *intKeyList) EncodeFields(w model.FieldWriter) {
	a := w.Array("records", len(*l))
	for i := range *l {
		a.Object(&(*l)[i])
	}
}
func (l *intKeyList) DecodeFields(r model.FieldReader) {
	if a, ok := r.Array("records"); ok {
		for i := 0; i < a.Len(); i++ {
			a.Object(i, l.Append().(model.Decodable))
		}
	}
}
func (l *intKeyList) IsNil() bool { return l == nil }

func TestStoredEntity_CompositeKeyWithInt(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &intKeyRec{},
		NewList: func() model.ModelSlice { return &intKeyList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)
	caller.Call(router.Operation{Module: "tmod", Entity: "intkey", Action: model.Create}, &crud.CreateArgs{Records: []model.Model{&intKeyRec{ShiftID: "s1", SpecificDate: 1700000000}}}, nil, func(e error) {})
	var list intKeyList
	var err error
	caller.Call(router.Operation{Module: "tmod", Entity: "intkey", Action: model.Read}, &crud.ReadArgs{ID: "s1/1700000000"}, &list, func(e error) { err = e })
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ShiftID != "s1" || list[0].SpecificDate != 1700000000 {
		t.Errorf("expected read, got %v", list)
	}
}

func TestStoredEntity_PublicCreateWithoutSession(t *testing.T) {
	reg := &mockRouter{}
	crud.Mount(reg, []crud.Public{{Model: &tRec{}, Actions: model.Create}}, nil, &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, model.Create),
		Create: func(c crud.CurrentUser, tx crud.Tx, rec model.Model) (crud.RuleAnswer, error) {
			return crud.ApplyAndPublish, nil
		},
	}})
	if !reg.Has("t_create") {
		t.Errorf("expected t_create mounted")
	}
}

func TestStoredEntity_ServerOnlyRejected(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)
	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{CreatedAt: 123}}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrServerOnlyField) {
		t.Errorf("expected ErrServerOnlyField, got %v", err)
	}
}
