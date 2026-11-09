package tests

import (
	"strings"
	"testing"

	"webtyp.com/crud"
	"webtyp.com/model"
	"webtyp.com/router"
)

type byNameFilter struct {
	Name string
}

func (f *byNameFilter) ModelName() string { return "by_name_filter" }
func (f *byNameFilter) IsNil() bool       { return f == nil }
func (f *byNameFilter) Schema() []model.Field {
	return []model.Field{
		{Name: "name", Type: model.Text(), NotNull: true},
	}
}
func (f *byNameFilter) Pointers() []any {
	return []any{&f.Name}
}
func (f *byNameFilter) EncodeFields(w model.FieldWriter) {
	w.String("name", f.Name)
}
func (f *byNameFilter) DecodeFields(r model.FieldReader) {
	f.Name, _ = r.String("name")
}

type filterTestModule struct {
	withFilter bool
	received   model.Model
}

func (m *filterTestModule) ModelName() string { return "filtertest" }
func (m *filterTestModule) MountOperations(reg crud.Registry) {
	var newFilter func() model.Model
	if m.withFilter {
		newFilter = func() model.Model { return &byNameFilter{} }
	}
	reg.AddReadOnlyEntity(&mockRecord{}, newFilter, func(c crud.CurrentUser, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		m.received = filter
		out := mockRecordList{}
		return &out, nil
	})
}
func (m *filterTestModule) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{Name: "filtertest", Entities: []model.Definition{{
		Name: "mock", Wire: true, Allow: model.Read, Rules: model.Rules{Read: model.AddMyBusinessRule}}}}
}

func TestRead_FilterDecodedAndValidated(t *testing.T) {
	mod := &filterTestModule{withFilter: true}
	caller := crud.Loopback(nil, mod)

	var out mockRecordList
	var err error
	readArgs := &crud.ReadArgs{
		Filter: &byNameFilter{Name: "ana"},
	}
	caller.Call(router.Operation{Module: "filtertest", Entity: "mock", Action: model.Read}, readArgs, &out, func(e error) {
		err = e
	})
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if mod.received == nil {
		t.Fatalf("expected received filter, got nil")
	}
	bnf, ok := mod.received.(*byNameFilter)
	if !ok || bnf.Name != "ana" {
		t.Fatalf("expected *byNameFilter with Name 'ana', got %+v", mod.received)
	}
}

func TestRead_FilterInvalidIs400(t *testing.T) {
	mod := &filterTestModule{withFilter: true}
	caller := crud.Loopback(nil, mod)

	var out mockRecordList
	var err error
	readArgs := &crud.ReadArgs{
		Filter: &byNameFilter{Name: ""}, // NotNull violation
	}
	caller.Call(router.Operation{Module: "filtertest", Entity: "mock", Action: model.Read}, readArgs, &out, func(e error) {
		err = e
	})
	if err == nil {
		t.Fatalf("expected error on invalid filter")
	}
	if model.KindOf(err) != model.ErrorInvalid {
		t.Fatalf("expected ErrorInvalid, got %v", model.KindOf(err))
	}
	if model.FieldOf(err) != "name" && !strings.Contains(err.Error(), "name") {
		t.Errorf("expected field or error mentioning 'name', got field=%q err=%v", model.FieldOf(err), err)
	}
	if mod.received != nil {
		t.Fatalf("read should not have been called, but got %+v", mod.received)
	}
}

func TestRead_NilFilter(t *testing.T) {
	mod := &filterTestModule{withFilter: false}
	caller := crud.Loopback(nil, mod)

	var out mockRecordList
	var err error
	caller.Call(router.Operation{Module: "filtertest", Entity: "mock", Action: model.Read}, &crud.ReadArgs{}, &out, func(e error) {
		err = e
	})
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if mod.received != nil {
		t.Fatalf("expected nil filter received, got %+v", mod.received)
	}
}
