package tests

import (
	"strings"
	"testing"

	"webtyp.com/crud"
	"webtyp.com/model"
)

type serverOnlyRecord struct {
	ID    string
	Name  string
	Owner string
}

func (r *serverOnlyRecord) ModelName() string { return "so_record" }
func (r *serverOnlyRecord) IsNil() bool       { return r == nil }
func (r *serverOnlyRecord) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text()},
		{Name: "name", Type: model.Text()},
		{Name: "owner", Type: model.Text(), ServerOnly: true, NotNull: true},
	}
}
func (r *serverOnlyRecord) Pointers() []any {
	return []any{&r.ID, &r.Name, &r.Owner}
}
func (r *serverOnlyRecord) EncodeFields(w model.FieldWriter) {
	w.String("id", r.ID)
	w.String("name", r.Name)
	w.String("owner", r.Owner)
}
func (r *serverOnlyRecord) DecodeFields(rdr model.FieldReader) {
	r.ID, _ = rdr.String("id")
	r.Name, _ = rdr.String("name")
	r.Owner, _ = rdr.String("owner")
}

type serverOnlyRecordList []*serverOnlyRecord

func (s *serverOnlyRecordList) Len() int                         { return len(*s) }
func (s *serverOnlyRecordList) At(i int) model.Fielder           { return (*s)[i] }
func (s *serverOnlyRecordList) Append() model.Fielder            { v := &serverOnlyRecord{}; *s = append(*s, v); return v }
func (s *serverOnlyRecordList) IsNil() bool                      { return s == nil }
func (s *serverOnlyRecordList) EncodeFields(_ model.FieldWriter) {}
func (s *serverOnlyRecordList) DecodeFields(_ model.FieldReader) {}

type serverOnlyModule struct {
	createdCalled bool
	updatedCalled bool
}

func (m *serverOnlyModule) ModelName() string { return "somod" }
func (m *serverOnlyModule) MountOperations(reg crud.Registry) {
	reg.Create(func() model.ModelSlice { return &serverOnlyRecordList{} }, func(c crud.Caller, list model.ModelSlice) error {
		m.createdCalled = true
		return nil
	})
	reg.Update(func() model.Model { return &serverOnlyRecord{} }, func(c crud.Caller, ids, fields []string, rec model.Model) error {
		m.updatedCalled = true
		return nil
	})
}

func TestServerOnly_CreateRejected(t *testing.T) {
	mod := &serverOnlyModule{}
	caller := crud.Loopback(mod)

	var out serverOnlyRecord
	var err error
	args := &crud.CreateArgs{
		Records: []model.Model{
			&serverOnlyRecord{Name: "a", Owner: "x"},
		},
	}
	caller.Call("somod."+crud.Op(&serverOnlyRecord{}, model.Create), args, &out, func(e error) {
		err = e
	})
	if err == nil {
		t.Fatalf("expected error when writing server-only field")
	}
	if !strings.Contains(err.Error(), "crud: field written only by the server") {
		t.Fatalf("expected error text 'crud: field written only by the server', got %v", err)
	}
	if mod.createdCalled {
		t.Fatalf("create should not have been called")
	}
}

func TestServerOnly_CreateEmptyAllowed(t *testing.T) {
	mod := &serverOnlyModule{}
	caller := crud.Loopback(mod)

	var out serverOnlyRecord
	var err error
	args := &crud.CreateArgs{
		Records: []model.Model{
			&serverOnlyRecord{Name: "a"}, // Owner is empty, even though NotNull
		},
	}
	caller.Call("somod."+crud.Op(&serverOnlyRecord{}, model.Create), args, &out, func(e error) {
		err = e
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if !mod.createdCalled {
		t.Fatalf("expected create to be called successfully")
	}
}

func TestServerOnly_UpdateRejected(t *testing.T) {
	mod := &serverOnlyModule{}
	caller := crud.Loopback(mod)

	var err error
	args := &crud.UpdateArgs{
		IDs:    []string{"1"},
		Fields: []string{"owner"},
		Record: &serverOnlyRecord{Owner: "y"},
	}
	caller.Call("somod."+crud.Op(&serverOnlyRecord{}, model.Update), args, nil, func(e error) {
		err = e
	})
	if err == nil {
		t.Fatalf("expected error when updating server-only field")
	}
	if !strings.Contains(err.Error(), "crud: field written only by the server") {
		t.Fatalf("expected error text 'crud: field written only by the server', got %v", err)
	}
	if mod.updatedCalled {
		t.Fatalf("update should not have been called")
	}
}
