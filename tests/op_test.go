package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/router"
)

func TestOperation_StoredEntityMountsOneOperationPerAllowedAction(t *testing.T) {
	reg := &mockRouter{}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &justRead{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.Create|model.Read, 0),
	}}
	err := crud.Mount(reg, nil, nil, mod)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !reg.Has("t_create") {
		t.Errorf("expected t_create mounted")
	}
	if !reg.Has("t_read") {
		t.Errorf("expected t_read mounted")
	}
	if reg.Has("t_update") {
		t.Errorf("t_update should not be mounted")
	}
	if reg.Has("t_delete") {
		t.Errorf("t_delete should not be mounted")
	}
}

type exactOpRouter struct {
	ops []router.Operation
}

func (r *exactOpRouter) Operation(op router.Operation, h router.HandlerFunc) router.Route {
	r.ops = append(r.ops, op)
	return routeStub{}
}

func TestOperation_WireReadMountsItsOwnOperation(t *testing.T) {
	reg := &exactOpRouter{}
	mod := &filterTestModule{withFilter: false}
	err := crud.Mount(reg, nil, nil, mod)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	expected := router.Operation{Module: "filtertest", Entity: "mock", Action: model.Read}
	found := false
	for _, op := range reg.ops {
		if op == expected {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected operation %+v to be mounted, got %v", expected, reg.ops)
	}
}

type namedModule struct {
	name  string
	table crud.StoredEntity
}

func (m *namedModule) ModelName() string { return m.name }
func (m *namedModule) MountOperations(reg crud.Registry) {
	reg.AddStoredEntity(m.table)
}
func (m *namedModule) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{Name: m.name, Entities: []model.Definition{{
		Name:  m.table.Model.ModelName(),
		Allow: m.table.Model.(model.Allower).Allow(),
		Rules: m.table.Rules,
	}}}
}

func TestOperation_ModuleIsPartOfTheIdentity(t *testing.T) {
	storeA := newMemStore()
	storeA.Create(&tRec{ID: "1", Name: "from_a"})
	modA := &namedModule{
		name: "a",
		table: crud.StoredEntity{
			Model:   &tRec{},
			NewList: func() model.ModelSlice { return &tRecList{} },
			Store:   storeA,
			Rules:   declareRules(model.AllActions, 0),
		},
	}

	storeB := newMemStore()
	storeB.Create(&tRec{ID: "1", Name: "from_b"})
	modB := &namedModule{
		name: "b",
		table: crud.StoredEntity{
			Model:   &tRec{},
			NewList: func() model.ModelSlice { return &tRecList{} },
			Store:   storeB,
			Rules:   declareRules(model.AllActions, 0),
		},
	}

	caller := crud.Loopback(nil, modA, modB)
	var list tRecList
	var err error
	caller.Call(router.Operation{Module: "a", Entity: "t", Action: model.Read}, &crud.ReadArgs{ID: "1"}, &list, func(e error) {
		err = e
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "from_a" {
		t.Errorf("expected 'from_a', got %v", list)
	}
}

func TestOperation_UnknownOperationIsAnError(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.Loopback(nil, mod)
	var err error
	caller.Call(router.Operation{Module: "nope", Entity: "t", Action: model.Read}, &crud.ReadArgs{}, nil, func(e error) {
		err = e
	})
	if err == nil {
		t.Fatal("expected error on unknown operation, got nil")
	}
	if !fmt.Contains(err.Error(), "nope.t.read") {
		t.Errorf("expected error containing 'nope.t.read', got %v", err)
	}
}
