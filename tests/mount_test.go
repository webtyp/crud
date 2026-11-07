package tests

import (
	"strings"
	"testing"

	"webtyp.com/crud"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/router/loopback"
)

type mockRecord struct {
	ID   string
	Name string
}

func (m *mockRecord) ModelName() string { return "test_record" }
func (m *mockRecord) IsNil() bool       { return m == nil }

func (m *mockRecord) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text()},
		{Name: "name", Type: model.Text()},
	}
}

func (m *mockRecord) Pointers() []any {
	return []any{&m.ID, &m.Name}
}

func (m *mockRecord) EncodeFields(w model.FieldWriter) {
	w.String("id", m.ID)
	w.String("name", m.Name)
}

func (m *mockRecord) DecodeFields(r model.FieldReader) {
	if val, ok := r.String("id"); ok {
		m.ID = val
	}
	if val, ok := r.String("name"); ok {
		m.Name = val
	}
}

type mockRecordList []*mockRecord

func (s *mockRecordList) Len() int                         { return len(*s) }
func (s *mockRecordList) At(i int) model.Fielder           { return (*s)[i] }
func (s *mockRecordList) Append() model.Fielder            { v := &mockRecord{}; *s = append(*s, v); return v }
func (s *mockRecordList) IsNil() bool                      { return s == nil }
func (s *mockRecordList) EncodeFields(_ model.FieldWriter) {}
func (s *mockRecordList) DecodeFields(_ model.FieldReader) {}

type testModule struct {
	store  map[string]*mockRecord
	create func(list mockRecordList) error
	read   func(args crud.ReadArgs) (mockRecordList, error)
	patch  func(ids, fields []string, rec mockRecord) error
	remove func(ids []string) error
}

func (m *testModule) ModelName() string { return "testmod" }
func (m *testModule) MountOperations(reg router.OperationRegistry) {
	crud.MountCreate(reg, func() model.ModelSlice { return &mockRecordList{} }, func(list model.ModelSlice) error {
		return m.create(*list.(*mockRecordList))
	})
	crud.MountRead(reg, &mockRecord{}, func(args crud.ReadArgs) (model.ModelSlice, error) {
		out, err := m.read(args)
		return &out, err
	})
	crud.MountUpdate(reg, func() model.Model { return &mockRecord{} }, func(ids, fields []string, rec model.Model) error {
		return m.patch(ids, fields, *rec.(*mockRecord))
	})
	crud.MountDelete(reg, &mockRecord{}, m.remove)
}

func opName(action model.Action) string {
	return "testmod." + crud.Op(&mockRecord{}, action)
}

func TestOp_HasNoDotsForRPCCallerCompatibility(t *testing.T) {
	for _, action := range []model.Action{model.Create, model.Read, model.Update, model.Delete} {
		bare := crud.Op(&mockRecord{}, action)
		if strings.Contains(bare, ".") {
			t.Fatalf("crud.Op(%v) = %q must not contain '.' because rpc.Caller requires exactly one '.' in <module>.<op>", action, bare)
		}
		if want := "test_record_" + action.String(); bare != want {
			t.Fatalf("crud.Op(%v) = %q, want %q", action, bare, want)
		}
	}
}

func TestMountCreate_RoundTripAndTransactionalBatch(t *testing.T) {
	var calls int
	var savedOrder []string
	mod := &testModule{
		store: make(map[string]*mockRecord),
	}
	mod.create = func(list mockRecordList) error {
		calls++
		// Single atomic transaction check across the whole batch before committing:
		for _, d := range list {
			if d.Name == "invalid" {
				return model.Conflict("dup")
			}
		}
		for _, d := range list {
			if d.ID == "" {
				d.ID = "generated"
			}
			savedOrder = append(savedOrder, d.ID)
			mod.store[d.ID] = d
		}
		return nil
	}
	c := loopback.ActingAs("t", "u", mod)

	// 1. Failing record in batch rolls back the entire batch (0 records stored)
	var err error
	c.Call(opName(model.Create), &crud.CreateArgs{Records: []model.Model{
		&mockRecord{ID: "1", Name: "one"},
		&mockRecord{ID: "2", Name: "invalid"},
	}}, nil, func(e error) { err = e })
	if err == nil || model.KindOf(err) != model.ErrorConflict {
		t.Fatalf("expected conflict on batch failure, got %v", err)
	}
	if len(mod.store) != 0 {
		t.Fatalf("expected transactional rollback (0 records stored), got %d", len(mod.store))
	}
	if calls != 1 {
		t.Fatalf("expected create handler called once for the batch, got %d", calls)
	}

	// 2. Valid batch commits all records in a single call
	out := &mockRecord{}
	c.Call(opName(model.Create), &crud.CreateArgs{Records: []model.Model{
		&mockRecord{ID: "1", Name: "one"},
		&mockRecord{Name: "second"},
	}}, out, func(e error) { err = e })

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected create handler called once for the second batch, got %d total", calls)
	}
	if len(mod.store) != 2 {
		t.Fatalf("expected 2 records saved, got %d", len(mod.store))
	}
	if mod.store["1"].Name != "one" || mod.store["generated"].Name != "second" {
		t.Fatalf("unexpected stored records: %v", mod.store)
	}
	if len(savedOrder) != 2 || savedOrder[0] != "1" || savedOrder[1] != "generated" {
		t.Fatalf("expected ids received in order [1, generated], got %v", savedOrder)
	}
	if out.ID != "generated" || out.Name != "second" {
		t.Fatalf("expected saved id filled on lastSaved out, got %v", out)
	}
}

func TestMountCreate_EmptyIs400(t *testing.T) {
	mod := &testModule{}
	c := loopback.ActingAs("t", "u", mod)
	var err error
	c.Call(opName(model.Create), &crud.CreateArgs{}, nil, func(e error) { err = e })

	if err == nil {
		t.Fatalf("expected error")
	}
	if model.KindOf(err) != model.ErrorInvalid || !strings.Contains(err.Error(), string(crud.ErrNoRecords)) {
		t.Fatalf("expected invalid %q, got %v", crud.ErrNoRecords, err)
	}
}

func TestMountRead_RoundTripAndCapsLimit(t *testing.T) {
	var gotArgs crud.ReadArgs
	mod := &testModule{
		read: func(args crud.ReadArgs) (mockRecordList, error) {
			gotArgs = args
			return mockRecordList{
				{ID: "1", Name: "Alpha"},
				{ID: "2", Name: "Beta"},
			}, nil
		},
	}
	c := loopback.ActingAs("t", "u", mod)
	var out mockRecordList
	var err error
	c.Call(opName(model.Read), &crud.ReadArgs{ID: "1", Search: "Al", After: "0", Limit: 9999}, &out, func(e error) { err = e })

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if gotArgs.ID != "1" || gotArgs.Search != "Al" || gotArgs.After != "0" || gotArgs.Limit != crud.MaxPage {
		t.Fatalf("expected capped ReadArgs, got %+v", gotArgs)
	}
	if len(out) != 2 || out[0].Name != "Alpha" || out[1].Name != "Beta" {
		t.Fatalf("unexpected read list: %+v", out)
	}
}

func TestMountDelete_RoundTrip(t *testing.T) {
	var deletedIDs []string
	mod := &testModule{
		remove: func(ids []string) error {
			deletedIDs = append(deletedIDs, ids...)
			return nil
		},
	}
	c := loopback.ActingAs("t", "u", mod)
	var err error
	c.Call(opName(model.Delete), &crud.DeleteArgs{IDs: []string{"1", "2"}}, nil, func(e error) { err = e })

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(deletedIDs) != 2 || deletedIDs[0] != "1" || deletedIDs[1] != "2" {
		t.Fatalf("expected deleted ids [1, 2], got %v", deletedIDs)
	}
}

func TestMountDelete_EmptyIs400(t *testing.T) {
	mod := &testModule{}
	c := loopback.ActingAs("t", "u", mod)
	var err error
	c.Call(opName(model.Delete), &crud.DeleteArgs{}, nil, func(e error) { err = e })

	if err == nil {
		t.Fatalf("expected error")
	}
	if model.KindOf(err) != model.ErrorInvalid || !strings.Contains(err.Error(), string(crud.ErrNoIDs)) {
		t.Fatalf("expected invalid %q, got %v", crud.ErrNoIDs, err)
	}
}

func TestMountUpdate_RoundTrip(t *testing.T) {
	var gotIDs, gotFields []string
	var gotRec mockRecord
	mod := &testModule{
		patch: func(ids, fields []string, rec mockRecord) error {
			gotIDs = append(gotIDs, ids...)
			gotFields = append(gotFields, fields...)
			gotRec = rec
			return nil
		},
	}
	c := loopback.ActingAs("t", "u", mod)
	var err error
	c.Call(opName(model.Update), &crud.UpdateArgs{
		IDs:    []string{"1", "2"},
		Fields: []string{"name"},
		Record: &mockRecord{Name: "patched"},
	}, nil, func(e error) { err = e })

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(gotIDs) != 2 || gotIDs[0] != "1" || gotIDs[1] != "2" {
		t.Fatalf("expected ids [1, 2], got %v", gotIDs)
	}
	if len(gotFields) != 1 || gotFields[0] != "name" {
		t.Fatalf("expected fields [name], got %v", gotFields)
	}
	if gotRec.Name != "patched" {
		t.Fatalf("expected carrier record Name=patched, got %+v", gotRec)
	}
}

func TestMountUpdate_EmptyIs400(t *testing.T) {
	mod := &testModule{}
	c := loopback.ActingAs("t", "u", mod)
	var err error
	c.Call(opName(model.Update), &crud.UpdateArgs{Fields: []string{"name"}, Record: &mockRecord{}}, nil, func(e error) { err = e })
	if err == nil || model.KindOf(err) != model.ErrorInvalid || !strings.Contains(err.Error(), string(crud.ErrNoIDs)) {
		t.Fatalf("expected invalid %q, got %v", crud.ErrNoIDs, err)
	}

	err = nil
	c.Call(opName(model.Update), &crud.UpdateArgs{IDs: []string{"1"}, Record: &mockRecord{}}, nil, func(e error) { err = e })
	if err == nil || model.KindOf(err) != model.ErrorInvalid || !strings.Contains(err.Error(), string(crud.ErrNoFields)) {
		t.Fatalf("expected invalid %q, got %v", crud.ErrNoFields, err)
	}
}

func TestMountUpdate_ErrorKeepsItsStatus(t *testing.T) {
	mod := &testModule{
		patch: func(ids, fields []string, rec mockRecord) error {
			return model.Conflict("locked")
		},
	}
	c := loopback.ActingAs("t", "u", mod)
	var err error
	c.Call(opName(model.Update), &crud.UpdateArgs{
		IDs:    []string{"1"},
		Fields: []string{"name"},
		Record: &mockRecord{Name: "x"},
	}, nil, func(e error) { err = e })

	if err == nil || model.KindOf(err) != model.ErrorConflict || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("expected conflict locked, got %v", err)
	}
}
