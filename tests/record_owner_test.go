package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/model"
)

type hiddenNoteRec struct{ tRec }

func (*hiddenNoteRec) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "name", Type: model.Text(), NotNull: true},
		{Name: "note", Type: model.Text(), HiddenFromRecordOwner: true},
		{Name: "created_at", Type: model.Int(), ServerOnly: true},
		{Name: "updated_at", Type: model.Int(), ServerOnly: true},
	}
}

func TestRecordOwner_HiddenFieldsOmittedForSeesOwnRecords(t *testing.T) {
	store := newMemStore()
	_ = store.Create(&tRec{ID: "r1", Name: "Lucia", Note: "internal clinic note"})

	mod := &testModule{table: crud.StoredEntity{
		Model:   &hiddenNoteRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   model.Rules{Create: model.HaveNoBusinessRule, Read: model.HaveNoBusinessRule, Update: model.HaveNoBusinessRule, Delete: model.HaveNoBusinessRule},
	}}
	callerOwner := crud.LoopbackActingAsUse(nil, "u1", model.SeesOwnRecords, []string{"r1"}, mod)
	var out tRecList
	var callErr error
	callerOwner.Call(operationOf(model.Read), &crud.ReadArgs{ID: "r1"}, &out, func(err error) { callErr = err })
	if callErr != nil {
		t.Fatal(callErr)
	}
	if len(out) != 1 || out[0].Note != "" {
		t.Fatalf("expected hidden note to be omitted for SeesOwnRecords, got %+v", out)
	}

	callerOwner.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r2", Name: "Ana", Note: "forbidden"}}}, nil, func(err error) { callErr = err })
	if callErr == nil {
		t.Fatal("expected error when SeesOwnRecords writes HiddenFromRecordOwner field")
	}

	callerStaff := crud.LoopbackActingAsUse(nil, "u2", model.WorksWithAllRecords, nil, mod)
	var staffOut tRecList
	callerStaff.Call(operationOf(model.Read), &crud.ReadArgs{ID: "r1"}, &staffOut, func(err error) { callErr = err })
	if callErr != nil {
		t.Fatal(callErr)
	}
	if len(staffOut) != 1 || staffOut[0].Note != "internal clinic note" {
		t.Fatalf("expected staff to see note, got %+v", staffOut)
	}
}

func TestRecordOwner_LoopbackActingAsUsePassesUseAndOwnedIDs(t *testing.T) {
	store := newMemStore()
	var gotUser crud.CurrentUser
	mod := &testModule{table: crud.StoredEntity{
		Model:   &hiddenNoteRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   model.Rules{Create: model.AddMyBusinessRule, Read: model.HaveNoBusinessRule, Update: model.HaveNoBusinessRule, Delete: model.HaveNoBusinessRule},
		Create: func(a crud.CurrentUser, _ crud.Tx, _ model.Model) (crud.RuleAnswer, error) {
			gotUser = a
			return crud.ApplyAndPublish(), nil
		},
	}}
	caller := crud.LoopbackActingAsUse(nil, "u1", model.SeesOwnRecords, []string{"c1", "c2"}, mod)
	var callErr error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "c3", Name: "Ana"}}}, nil, func(err error) { callErr = err })
	if callErr != nil {
		t.Fatal(callErr)
	}
	if gotUser.UserID != "u1" || gotUser.Use != model.SeesOwnRecords || len(gotUser.OwnedRecordIDs) != 2 {
		t.Fatalf("unexpected CurrentUser: %+v", gotUser)
	}
}
