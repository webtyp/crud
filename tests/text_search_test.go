package tests

import (
	"errors"
	"testing"

	"webtyp.com/crud"
	"webtyp.com/model"
	"webtyp.com/storage"
)

// searchText is a text kind that enters text_search (webtyp.com/input does this for Text, Phone, Email, Rut).
type searchText struct{ model.Kind }

func (searchText) SearchableText() {}

// searchFirstRec is tRec declared with ListsOnlyAfterSearch (definitionc generates this method).
type searchFirstRec struct{ tRec }

func (*searchFirstRec) ListsOnlyAfterSearch() bool { return true }

// noTextRec is fRec (no field enters text_search) declared with ListsOnlyAfterSearch.
type noTextRec struct{ fRec }

func (*noTextRec) ListsOnlyAfterSearch() bool { return true }

func searchModule(m model.Model, store crud.Store, search func(crud.CurrentUser, model.Model) ([]storage.Condition, error)) *testModule {
	domain := model.Action(0)
	if search != nil {
		domain = model.Read
	}
	return &testModule{table: crud.StoredEntity{
		Model:   m,
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, domain),
		Search:  search,
	}}
}

func readNames(t *testing.T, mod *testModule, args *crud.ReadArgs) []string {
	t.Helper()
	var list tRecList
	var err error
	crud.LoopbackActingAs(nil, "u1", mod).Call(operationOf(model.Read), args, &list, func(e error) { err = e })
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(list))
	for i, r := range list {
		names[i] = r.Name
	}
	return names
}

func TestTextSearch_IgnoresAccentsAndCase(t *testing.T) {
	store := newMemStore()
	store.Create(&tRec{ID: "1", Name: "Lucía Pérez"})
	store.Create(&tRec{ID: "2", Name: "Bruno Díaz"})
	got := readNames(t, searchModule(&tRec{}, store, nil), &crud.ReadArgs{Search: "LUCIA"})
	if len(got) != 1 || got[0] != "Lucía Pérez" {
		t.Errorf("got %v, want [Lucía Pérez]", got)
	}
}

func TestTextSearch_EveryWordMustAppear(t *testing.T) {
	store := newMemStore()
	store.Create(&tRec{ID: "1", Name: "Lucía Pérez"})
	store.Create(&tRec{ID: "2", Name: "Lucía Soto"})
	got := readNames(t, searchModule(&tRec{}, store, nil), &crud.ReadArgs{Search: "lucia soto"})
	if len(got) != 1 || got[0] != "Lucía Soto" {
		t.Errorf("got %v, want [Lucía Soto]", got)
	}
}

func TestTextSearch_RuleAddsConditionsNeverReplacesText(t *testing.T) {
	store := newMemStore()
	store.Create(&tRec{ID: "1", Name: "Ana", Note: "active"})
	store.Create(&tRec{ID: "2", Name: "Ana", Note: "retired"})
	store.Create(&tRec{ID: "3", Name: "Bea", Note: "active"})
	rule := func(crud.CurrentUser, model.Model) ([]storage.Condition, error) {
		return []storage.Condition{storage.Eq("note", "active")}, nil
	}
	got := readNames(t, searchModule(&tRec{}, store, rule), &crud.ReadArgs{Search: "ana"})
	if len(got) != 1 || got[0] != "Ana" {
		t.Errorf("got %v, want the one active Ana", got)
	}
}

func TestTextSearch_ListsOnlyAfterSearchReturnsNothingWithoutText(t *testing.T) {
	store := newMemStore()
	store.Create(&tRec{ID: "1", Name: "Ana"})
	if got := readNames(t, searchModule(&searchFirstRec{}, store, nil), &crud.ReadArgs{}); len(got) != 0 {
		t.Errorf("got %v, want no rows without text", got)
	}
	if got := readNames(t, searchModule(&searchFirstRec{}, store, nil), &crud.ReadArgs{Search: "ana"}); len(got) != 1 {
		t.Errorf("got %v, want Ana with text", got)
	}
}

func TestTextSearch_ListsOnlyAfterSearchNeedsATextField(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &noTextRec{},
		NewList: func() model.ModelSlice { return &fRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.Read, 0),
	}}
	if err := crud.Mount(nil, nil, nil, mod); !errors.Is(err, crud.ErrSearchFirstWithoutText) {
		t.Errorf("err = %v, want ErrSearchFirstWithoutText", err)
	}
}
