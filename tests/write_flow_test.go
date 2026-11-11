package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/fmt"
	"webtyp.com/model"
)

type recordingFieldWriter struct {
	objects []string // names passed to Object
	arrays  []string // names passed to Array
	strs    []string // strings pushed into any array
}

func (w *recordingFieldWriter) String(name, val string)        {}
func (w *recordingFieldWriter) Int(name string, val int64)     {}
func (w *recordingFieldWriter) Float(name string, val float64) {}
func (w *recordingFieldWriter) Bool(name string, val bool)     {}
func (w *recordingFieldWriter) Bytes(name string, val []byte)  {}
func (w *recordingFieldWriter) Null(name string)               {}
func (w *recordingFieldWriter) Raw(name, val string)           {}
func (w *recordingFieldWriter) Object(name string, val model.Encodable) {
	w.objects = append(w.objects, name)
}
func (w *recordingFieldWriter) Array(name string, n int) model.ArrayWriter {
	w.arrays = append(w.arrays, name)
	return &recordingArray{parent: w}
}

type recordingArray struct{ parent *recordingFieldWriter }

func (a *recordingArray) String(val string)          { a.parent.strs = append(a.parent.strs, val) }
func (a *recordingArray) Int(val int64)              {}
func (a *recordingArray) Float(val float64)          {}
func (a *recordingArray) Bool(val bool)              {}
func (a *recordingArray) Bytes(val []byte)           {}
func (a *recordingArray) Object(val model.Encodable) {}
func (a *recordingArray) Close()                     {}

func TestWrite_AnnounceIsPublishedAfterCommit(t *testing.T) {
	store := newMemStore()
	rec := &eventRecorder{}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, model.Create),
		Create: func(a crud.CurrentUser, tx crud.Tx, r model.Model) (crud.RuleAnswer, error) {
			return crud.ApplyAndPublish(), nil
		},
	}}
	caller := crud.LoopbackActingAs(rec, "u1", mod)
	var saved tRec
	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r1", Name: "saved"}}}, &saved, func(e error) { err = e })
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(rec.got))
	}
	if rec.got[0].Topic != "tmod.t.created" {
		t.Errorf("expected topic tmod.t.created, got %s", rec.got[0].Topic)
	}
	ch, ok := rec.got[0].Payload.(*crud.Change)
	if !ok {
		t.Fatalf("expected payload *crud.Change, got %T", rec.got[0].Payload)
	}
	if ch.Record.(*tRec).ID != "r1" {
		t.Errorf("expected id r1, got %s", ch.Record.(*tRec).ID)
	}
	if ch.Fields != nil {
		t.Errorf("expected nil Fields, got %v", ch.Fields)
	}
}

func TestWrite_SilentStoresWithoutPublishing(t *testing.T) {
	store := newMemStore()
	rec := &eventRecorder{}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, model.Create),
		Create: func(a crud.CurrentUser, tx crud.Tx, r model.Model) (crud.RuleAnswer, error) {
			return crud.ApplyWithoutPublishing(), nil
		},
	}}
	caller := crud.LoopbackActingAs(rec, "u1", mod)
	var saved tRec
	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r1"}}}, &saved, func(e error) { err = e })
	if err != nil {
		t.Fatal(err)
	}
	if len(store.rows) != 1 {
		t.Errorf("expected 1 stored row, got %d", len(store.rows))
	}
	if len(rec.got) != 0 {
		t.Errorf("expected 0 published events, got %d", len(rec.got))
	}
}

func TestWrite_StandardAlwaysAnnounces(t *testing.T) {
	store := newMemStore()
	rec := &eventRecorder{}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(rec, "u1", mod)
	var saved tRec
	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r1"}}}, &saved, func(e error) { err = e })
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(rec.got))
	}
	if rec.got[0].Topic != "tmod.t.created" {
		t.Errorf("expected topic tmod.t.created, got %s", rec.got[0].Topic)
	}
}

func TestWrite_RuleErrorPublishesNothing(t *testing.T) {
	store := newMemStore()
	rec := &eventRecorder{}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, model.Create),
		Create: func(a crud.CurrentUser, tx crud.Tx, r model.Model) (crud.RuleAnswer, error) {
			return crud.RuleAnswer{}, model.Conflict("no")
		},
	}}
	caller := crud.LoopbackActingAs(rec, "u1", mod)
	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r1"}}}, nil, func(e error) { err = e })
	if err == nil || !fmt.Contains(err.Error(), "no") {
		t.Errorf("expected error containing 'no', got %v", err)
	}
	if len(rec.got) != 0 {
		t.Errorf("expected 0 events, got %d", len(rec.got))
	}
	if len(store.rows) != 0 {
		t.Errorf("expected empty store, got %d rows", len(store.rows))
	}
}

func TestWrite_ZeroResultIsRefused(t *testing.T) {
	store := newMemStore()
	rec := &eventRecorder{}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, model.Create),
		Create: func(a crud.CurrentUser, tx crud.Tx, r model.Model) (crud.RuleAnswer, error) {
			return crud.RuleAnswer{}, nil
		},
	}}
	caller := crud.LoopbackActingAs(rec, "u1", mod)
	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r1"}}}, nil, func(e error) { err = e })
	if err == nil {
		t.Fatal("expected error on zero RuleAnswer, got nil")
	}
	if fmt.Contains(err.Error(), "answered nothing") {
		t.Errorf("generic client error must not leak internal detail, got %v", err)
	}
	if len(rec.got) != 0 {
		t.Errorf("expected 0 events, got %d", len(rec.got))
	}
	if len(store.rows) != 0 {
		t.Errorf("expected empty store (rolled back), got %d rows", len(store.rows))
	}
}

func TestWrite_BatchRollbackPublishesNothing(t *testing.T) {
	store := newMemStore()
	rec := &eventRecorder{}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, model.Create),
		Create: func(a crud.CurrentUser, tx crud.Tx, r model.Model) (crud.RuleAnswer, error) {
			if r.(*tRec).ID == "r2" {
				return crud.RuleAnswer{}, model.Conflict("r2 rejected")
			}
			return crud.ApplyAndPublish(), nil
		},
	}}
	caller := crud.LoopbackActingAs(rec, "u1", mod)
	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r1"}, &tRec{ID: "r2"}}}, nil, func(e error) { err = e })
	if err == nil {
		t.Fatal("expected batch error, got nil")
	}
	if len(store.rows) != 0 {
		t.Errorf("expected 0 rows in store (rolled back), got %d", len(store.rows))
	}
	if len(rec.got) != 0 {
		t.Errorf("expected 0 published events, got %d", len(rec.got))
	}
}

func TestWrite_UpdateEventCarriesFieldsAndEachID(t *testing.T) {
	store := newMemStore()
	store.Create(&tRec{ID: "a", Name: "old_a"})
	store.Create(&tRec{ID: "b", Name: "old_b"})
	rec := &eventRecorder{}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(rec, "u1", mod)
	var err error
	caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: []string{"a", "b"}, Fields: []string{"name"}, Record: &tRec{Name: "new"}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(rec.got))
	}
	if rec.got[0].Topic != "tmod.t.updated" || rec.got[1].Topic != "tmod.t.updated" {
		t.Errorf("expected topic tmod.t.updated, got %s and %s", rec.got[0].Topic, rec.got[1].Topic)
	}
	ch0 := rec.got[0].Payload.(*crud.Change)
	ch1 := rec.got[1].Payload.(*crud.Change)
	if ch0.Record.(*tRec).ID != "a" || ch1.Record.(*tRec).ID != "b" {
		t.Errorf("expected IDs 'a' and 'b', got %s and %s", ch0.Record.(*tRec).ID, ch1.Record.(*tRec).ID)
	}
	if len(ch0.Fields) != 1 || ch0.Fields[0] != "name" {
		t.Errorf("expected Fields ['name'], got %v", ch0.Fields)
	}
	if len(ch1.Fields) != 1 || ch1.Fields[0] != "name" {
		t.Errorf("expected Fields ['name'], got %v", ch1.Fields)
	}
}

func TestWrite_UpdateSilentForOneID(t *testing.T) {
	store := newMemStore()
	store.Create(&tRec{ID: "a", Name: "old_a"})
	store.Create(&tRec{ID: "b", Name: "old_b"})
	rec := &eventRecorder{}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, model.Update),
		Update: func(a crud.CurrentUser, tx crud.Tx, old, next model.Model, fields []string) (crud.RuleAnswer, error) {
			if old.(*tRec).ID == "a" {
				return crud.ApplyWithoutPublishing(), nil
			}
			return crud.ApplyAndPublish(), nil
		},
	}}
	caller := crud.LoopbackActingAs(rec, "u1", mod)
	var err error
	caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: []string{"a", "b"}, Fields: []string{"name"}, Record: &tRec{Name: "next"}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(rec.got))
	}
	ch := rec.got[0].Payload.(*crud.Change)
	if ch.Record.(*tRec).ID != "b" {
		t.Errorf("expected event for 'b', got %s", ch.Record.(*tRec).ID)
	}
}

func TestWrite_DeleteEventCarriesStoredRow(t *testing.T) {
	store := newMemStore()
	store.Create(&tRec{ID: "a", Name: "x"})
	rec := &eventRecorder{}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(rec, "u1", mod)
	var err error
	caller.Call(operationOf(model.Delete), &crud.DeleteArgs{IDs: []string{"a"}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(rec.got))
	}
	if rec.got[0].Topic != "tmod.t.deleted" {
		t.Errorf("expected topic tmod.t.deleted, got %s", rec.got[0].Topic)
	}
	ch := rec.got[0].Payload.(*crud.Change)
	if ch.Record.(*tRec).Name != "x" {
		t.Errorf("expected Record Name 'x', got %s", ch.Record.(*tRec).Name)
	}
	if ch.Fields != nil {
		t.Errorf("expected nil Fields, got %v", ch.Fields)
	}
}

func TestWrite_NoPublisherIsFine(t *testing.T) {
	store := newMemStore()
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)
	var saved tRec
	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r1"}}}, &saved, func(e error) { err = e })
	if err != nil {
		t.Fatal(err)
	}
	if len(store.rows) != 1 {
		t.Errorf("expected 1 stored row, got %d", len(store.rows))
	}
}

func TestWrite_ReadNeverPublishes(t *testing.T) {
	store := newMemStore()
	store.Create(&tRec{ID: "r1", Name: "one"})
	rec := &eventRecorder{}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(rec, "u1", mod)
	var list tRecList
	var err error
	caller.Call(operationOf(model.Read), &crud.ReadArgs{}, &list, func(e error) { err = e })
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.got) != 0 {
		t.Errorf("expected 0 published events for read, got %d", len(rec.got))
	}
}

func TestTopic_Shape(t *testing.T) {
	if got := crud.Topic("agenda", "patient", model.Create); got != "agenda.patient.created" {
		t.Errorf("expected agenda.patient.created, got %s", got)
	}
	if got := crud.Topic("agenda", "patient", model.Update); got != "agenda.patient.updated" {
		t.Errorf("expected agenda.patient.updated, got %s", got)
	}
	if got := crud.Topic("agenda", "patient", model.Delete); got != "agenda.patient.deleted" {
		t.Errorf("expected agenda.patient.deleted, got %s", got)
	}
	if got := crud.Topic("agenda", "patient", model.Read); got != "" {
		t.Errorf("expected empty topic for read, got %s", got)
	}
}

func TestChange_EncodesRecordAndFields(t *testing.T) {
	ch := &crud.Change{
		Record: &tRec{ID: "a"},
		Fields: []string{"name"},
	}
	w := &recordingFieldWriter{}
	ch.EncodeFields(w)
	if len(w.objects) != 1 || w.objects[0] != "record" {
		t.Errorf("expected Object('record'), got %v", w.objects)
	}
	if len(w.arrays) != 1 || w.arrays[0] != "fields" {
		t.Errorf("expected Array('fields'), got %v", w.arrays)
	}
	if len(w.strs) != 1 || w.strs[0] != "name" {
		t.Errorf("expected String('name'), got %v", w.strs)
	}

	w2 := &recordingFieldWriter{}
	chNoFields := &crud.Change{Record: &tRec{ID: "b"}}
	chNoFields.EncodeFields(w2)
	if len(w2.objects) != 1 || w2.objects[0] != "record" {
		t.Errorf("expected Object('record'), got %v", w2.objects)
	}
	if len(w2.arrays) != 0 {
		t.Errorf("expected no arrays when Fields is empty, got %v", w2.arrays)
	}
}
