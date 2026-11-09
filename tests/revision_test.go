package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/storage"
)

type rRec struct {
	ID        string
	Name      string
	Note      string
	CreatedAt int64
	UpdatedAt int64
	Revision  int64
}

func (m *rRec) ModelName() string        { return "r" }
func (m *rRec) Allow() model.Action      { return model.AllActions }
func (m *rRec) IsNil() bool              { return m == nil }
func (m *rRec) EncodeFields(w model.FieldWriter) {
	w.String("id", m.ID)
	w.String("name", m.Name)
	w.String("note", m.Note)
	w.Int("created_at", m.CreatedAt)
	w.Int("updated_at", m.UpdatedAt)
	w.Int("revision", m.Revision)
}
func (m *rRec) DecodeFields(r model.FieldReader) {
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
	if v, ok := r.Int("revision"); ok {
		m.Revision = v
	}
}
func (m *rRec) Pointers() []any {
	return []any{&m.ID, &m.Name, &m.Note, &m.CreatedAt, &m.UpdatedAt, &m.Revision}
}
func (m *rRec) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true, AutoInc: true}},
		{Name: "name", Type: model.Text(), Item: model.ItemLabel},
		{Name: "note", Type: model.Text()},
		{Name: "created_at", Type: model.Int(), ServerOnly: true},
		{Name: "updated_at", Type: model.Int(), ServerOnly: true},
		{Name: "revision", Type: model.Int(), DB: &model.FieldDB{Revision: true}},
	}
}

type rRecList []rRec

func (l *rRecList) Append() model.Fielder { *l = append(*l, rRec{}); return &(*l)[len(*l)-1] }
func (l *rRecList) Len() int             { return len(*l) }
func (l *rRecList) At(i int) model.Fielder { return &(*l)[i] }
func (l *rRecList) EncodeFields(w model.FieldWriter) {
	a := w.Array("records", len(*l))
	for i := range *l {
		a.Object(&(*l)[i])
	}
}
func (l *rRecList) DecodeFields(r model.FieldReader) {
	if a, ok := r.Array("records"); ok {
		for i := 0; i < a.Len(); i++ {
			f := l.Append().(model.Decodable)
			a.Object(i, f)
		}
	}
}
func (l *rRecList) IsNil() bool { return l == nil }

func rOperationOf(a model.Action) router.Operation {
	return router.Operation{Module: "tmod", Entity: "r", Action: a}
}

func TestRevision_CreateStartsAtOne(t *testing.T) {
	store := newMemStore()
	mod := &testModule{table: crud.StoredEntity{
		Model:   &rRec{},
		NewList: func() model.ModelSlice { return &rRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)

	var saved rRec
	var err error
	caller.Call(rOperationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&rRec{Name: "rev1"}}}, &saved, func(e error) { err = e })
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if saved.Revision != 1 {
		t.Errorf("expected Revision == 1, got %d", saved.Revision)
	}

	var stored rRec
	if err := store.ReadOne(&stored, saved.ID); err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if stored.Revision != 1 {
		t.Errorf("stored Revision: expected 1, got %d", stored.Revision)
	}
}

func TestRevision_UpdateBumpsTheRevision(t *testing.T) {
	store := newMemStore()
	rec := &eventRecorder{}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &rRec{},
		NewList: func() model.ModelSlice { return &rRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(rec, "u1", mod)

	var saved rRec
	var err error
	caller.Call(rOperationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&rRec{Name: "first"}}}, &saved, func(e error) { err = e })
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	caller.Call(rOperationOf(model.Update), &crud.UpdateArgs{IDs: []string{saved.ID}, Fields: []string{"name"}, Record: &rRec{Name: "updated", Revision: 1}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	var stored rRec
	if err := store.ReadOne(&stored, saved.ID); err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if stored.Revision != 2 {
		t.Errorf("expected stored Revision == 2, got %d", stored.Revision)
	}

	if len(rec.got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(rec.got))
	}
	ch, ok := rec.got[1].Payload.(*crud.Change)
	if !ok {
		t.Fatalf("expected *crud.Change, got %T", rec.got[1].Payload)
	}
	eventRec, ok := ch.Record.(*rRec)
	if !ok {
		t.Fatalf("expected *rRec, got %T", ch.Record)
	}
	if eventRec.Revision != 2 {
		t.Errorf("expected event record Revision == 2, got %d", eventRec.Revision)
	}
}

func TestRevision_StaleUpdateIsAConflict(t *testing.T) {
	store := newMemStore()
	mod := &testModule{table: crud.StoredEntity{
		Model:   &rRec{},
		NewList: func() model.ModelSlice { return &rRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)

	var saved rRec
	var err error
	caller.Call(rOperationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&rRec{Name: "first"}}}, &saved, func(e error) { err = e })
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// First update with Revision 1 -> OK (becomes 2)
	caller.Call(rOperationOf(model.Update), &crud.UpdateArgs{IDs: []string{saved.ID}, Fields: []string{"name"}, Record: &rRec{Name: "updated_ok", Revision: 1}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("first update failed: %v", err)
	}

	// Stale update with Revision 1 again -> 409 Conflict
	caller.Call(rOperationOf(model.Update), &crud.UpdateArgs{IDs: []string{saved.ID}, Fields: []string{"name"}, Record: &rRec{Name: "stale_mod", Revision: 1}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 409 "+string(crud.ErrConflict) {
		t.Errorf("expected 409 ErrConflict, got %v", err)
	}

	var stored rRec
	store.ReadOne(&stored, saved.ID)
	if stored.Name != "updated_ok" {
		t.Errorf("expected stored Name 'updated_ok', got %q", stored.Name)
	}
	if stored.Revision != 2 {
		t.Errorf("expected stored Revision 2, got %d", stored.Revision)
	}
}

func TestRevision_UpdateWithoutRevisionIsRefused(t *testing.T) {
	store := newMemStore()
	mod := &testModule{table: crud.StoredEntity{
		Model:   &rRec{},
		NewList: func() model.ModelSlice { return &rRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)

	var saved rRec
	var err error
	caller.Call(rOperationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&rRec{Name: "first"}}}, &saved, func(e error) { err = e })
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	caller.Call(rOperationOf(model.Update), &crud.UpdateArgs{IDs: []string{saved.ID}, Fields: []string{"name"}, Record: &rRec{Name: "no_rev", Revision: 0}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrRevisionRequired) {
		t.Errorf("expected ErrRevisionRequired, got %v", err)
	}
}

func TestRevision_ClientCannotWriteTheRevision(t *testing.T) {
	store := newMemStore()
	mod := &testModule{table: crud.StoredEntity{
		Model:   &rRec{},
		NewList: func() model.ModelSlice { return &rRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)

	var saved rRec
	var err error
	caller.Call(rOperationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&rRec{Name: "first"}}}, &saved, func(e error) { err = e })
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	caller.Call(rOperationOf(model.Update), &crud.UpdateArgs{IDs: []string{saved.ID}, Fields: []string{"revision"}, Record: &rRec{Name: "bad", Revision: 1}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrInvalidField) {
		t.Errorf("expected ErrInvalidField, got %v", err)
	}
}

func TestRevision_StoreWithoutConditionalUpdateFailsMount(t *testing.T) {
	type plainStore struct{ crud.Store }
	store := &plainStore{Store: newMemStore()}
	err := crud.Mount(nil, nil, nil, &testModule{table: crud.StoredEntity{
		Model:   &rRec{},
		NewList: func() model.ModelSlice { return &rRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, 0),
	}})
	if err == nil || !fmt.Contains(err.Error(), "ConditionalUpdater") {
		t.Errorf("expected error containing ConditionalUpdater, got %v", err)
	}
}

type dupStore struct {
	*memStore
}

func (s *dupStore) Create(rec model.Model) error {
	return storage.Duplicate(errStoreDown)
}

func (s *dupStore) UpdateFields(rec model.Model, id string, fields []string) error {
	return storage.Duplicate(errStoreDown)
}

func (s *dupStore) Atomic(fn func(crud.Tx) error) error {
	return fn(s)
}

func TestWrite_DuplicateKeyIsAlreadyExists(t *testing.T) {
	ds := &dupStore{memStore: newMemStore()}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   ds,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)

	// Create
	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{}}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 409 "+string(crud.ErrAlreadyExists) {
		t.Errorf("create: expected ErrAlreadyExists, got %v", err)
	}

	// Update
	ds.memStore.Create(&tRec{ID: "r1", Name: "orig"})
	caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: []string{"r1"}, Fields: []string{"name"}, Record: &tRec{Name: "x"}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 409 "+string(crud.ErrAlreadyExists) {
		t.Errorf("update: expected ErrAlreadyExists, got %v", err)
	}
}
