package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/storage"
)

func TestMount_DomainWithoutHookIsOutOfDate(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, model.Create),
		Create:  nil, // missing hook
	}}
	err := crud.Mount(nil, nil, nil, mod)
	if err == nil {
		t.Fatal("expected out of date error, got nil")
	}
	if !fmt.Contains(err.Error(), "definitionc") || !fmt.Contains(err.Error(), "t") {
		t.Errorf("expected error containing 'definitionc' and 't', got %v", err)
	}
}

func TestMount_HookWithoutDomainIsOutOfDate(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, 0),
		Create: func(a crud.CurrentUser, tx crud.Tx, rec model.Model) (crud.RuleAnswer, error) {
			return crud.ApplyAndPublish, nil
		},
	}}
	err := crud.Mount(nil, nil, nil, mod)
	if err == nil {
		t.Fatal("expected out of date error, got nil")
	}
	if !fmt.Contains(err.Error(), "definitionc") {
		t.Errorf("expected error containing 'definitionc', got %v", err)
	}
}

func TestMount_SearchHookFollowsReadRule(t *testing.T) {
	// 1. Rules.Read is HaveNoBusinessRule, but Search hook is set
	mod1 := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, 0),
		Search: func(a crud.CurrentUser, text string) ([]storage.Condition, error) {
			return nil, nil
		},
	}}
	err1 := crud.Mount(nil, nil, nil, mod1)
	if err1 == nil || !fmt.Contains(err1.Error(), "definitionc") {
		t.Errorf("expected definitionc error when Search is set without AddMyBusinessRule, got %v", err1)
	}

	// 2. Rules.Read is AddMyBusinessRule, but Search hook is nil
	mod2 := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, model.Read),
		Search:  nil,
	}}
	err2 := crud.Mount(nil, nil, nil, mod2)
	if err2 == nil || !fmt.Contains(err2.Error(), "definitionc") {
		t.Errorf("expected definitionc error when Search is nil with AddMyBusinessRule, got %v", err2)
	}
}

func TestMount_AllowedActionWithoutRuleIsRefused(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   model.Rules{}, // zero rules with Allow() == AllActions
	}}
	err := crud.Mount(nil, nil, nil, mod)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !fmt.Contains(err.Error(), "must declare its rule") {
		t.Errorf("expected error containing 'must declare its rule', got %v", err)
	}
}

type multiEntityModule struct {
	name   string
	def    model.ModuleDefinition
	mounts func(reg crud.Registry)
}

func (m *multiEntityModule) ModelName() string                 { return m.name }
func (m *multiEntityModule) Definition() model.ModuleDefinition { return m.def }
func (m *multiEntityModule) MountOperations(reg crud.Registry) {
	if m.mounts != nil {
		m.mounts(reg)
	}
}

func TestMount_HandlerListedButNotMounted(t *testing.T) {
	mod := &multiEntityModule{
		name: "tmod",
		def: model.ModuleDefinition{
			Name: "tmod",
			Entities: []model.Definition{
				{Name: "t", Allow: model.Create, Rules: model.Rules{Create: model.HaveNoBusinessRule}},
				{Name: "u", Allow: model.Create, Rules: model.Rules{Create: model.HaveNoBusinessRule}},
			},
		},
		mounts: func(reg crud.Registry) {
			reg.AddStoredEntity(crud.StoredEntity{
				Model:   &tRec{},
				NewList: func() model.ModelSlice { return &tRecList{} },
				Store:   newMemStore(),
				Rules:   declareRules(model.AllActions, 0),
			})
		},
	}
	err := crud.Mount(nil, nil, nil, mod)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !fmt.Contains(err.Error(), "definitionc") || !fmt.Contains(err.Error(), "u") {
		t.Errorf("expected error containing 'definitionc' and 'u', got %v", err)
	}
}

func TestMount_MountedButNotListed(t *testing.T) {
	mod := &multiEntityModule{
		name: "tmod",
		def: model.ModuleDefinition{
			Name:     "tmod",
			Entities: []model.Definition{}, // empty
		},
		mounts: func(reg crud.Registry) {
			reg.AddStoredEntity(crud.StoredEntity{
				Model:   &tRec{},
				NewList: func() model.ModelSlice { return &tRecList{} },
				Store:   newMemStore(),
				Rules:   declareRules(model.AllActions, 0),
			})
		},
	}
	err := crud.Mount(nil, nil, nil, mod)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !fmt.Contains(err.Error(), "definitionc") || !fmt.Contains(err.Error(), "t") {
		t.Errorf("expected error containing 'definitionc' and 't', got %v", err)
	}
}

func TestMount_DefinitionNameMustMatchModelName(t *testing.T) {
	mod := &multiEntityModule{
		name: "tmod",
		def: model.ModuleDefinition{
			Name: "other",
		},
		mounts: func(reg crud.Registry) {},
	}
	err := crud.Mount(nil, nil, nil, mod)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !fmt.Contains(err.Error(), "definitionc") {
		t.Errorf("expected error containing 'definitionc', got %v", err)
	}
}

func TestMount_InternalHandlerIsNotMounted(t *testing.T) {
	mod := &multiEntityModule{
		name: "tmod",
		def: model.ModuleDefinition{
			Name: "tmod",
			Entities: []model.Definition{
				{Name: "internal", Allow: 0},
			},
		},
		mounts: func(reg crud.Registry) {},
	}
	reg := &mockRouter{}
	err := crud.Mount(reg, nil, nil, mod)
	if err != nil {
		t.Errorf("expected nil error for internal entity, got %v", err)
	}
}

func TestMount_ConsistentModuleMounts(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, model.Create),
		Create: func(a crud.CurrentUser, tx crud.Tx, rec model.Model) (crud.RuleAnswer, error) {
			return crud.ApplyAndPublish, nil
		},
	}}
	reg := &mockRouter{}
	err := crud.Mount(reg, nil, nil, mod)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !reg.Has("t_create") {
		t.Errorf("expected reg.Has('t_create') to be true")
	}
}
