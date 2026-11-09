package conformance

import (
	"testing"
	"webtyp.com/crud"
	"webtyp.com/model"
	"webtyp.com/storage"
)

type TestModel struct {
	ID        string
	Name      string
	CreatedAt int64
	UpdatedAt int64
}

func (m *TestModel) IsNil() bool { return m == nil }
func (m *TestModel) EncodeFields(w model.FieldWriter) {}
func (m *TestModel) DecodeFields(r model.FieldReader) {}
func (m *TestModel) Pointers() []any { return []any{&m.ID, &m.Name, &m.CreatedAt, &m.UpdatedAt} }
func (m *TestModel) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "name", Type: model.Text(), Item: model.ItemLabel},
		{Name: "created_at", Type: model.Int()},
		{Name: "updated_at", Type: model.Int()},
	}
}
func (m *TestModel) ModelName() string { return "test_model" }
func (m *TestModel) Allow() model.Action { return model.AllActions }

func Run(t *testing.T, newStore func(t *testing.T) crud.Store) {
	t.Run("ReadOneNotFound", func(t *testing.T) {
		s := newStore(t)
		m := &TestModel{}
		err := s.ReadOne(m, "nope")
		if model.KindOf(err) != model.ErrorNotFound {
			t.Errorf("expected ErrorNotFound, got %v", err)
		}
	})
	t.Run("CreateReadOneRoundTrips", func(t *testing.T) {
		s := newStore(t)
		m := &TestModel{ID: "m1", Name: "A"}
		if err := s.Create(m); err != nil {
			t.Fatal(err)
		}
		m2 := &TestModel{}
		if err := s.ReadOne(m2, "m1"); err != nil {
			t.Fatal(err)
		}
		if m2.Name != "A" {
			t.Errorf("expected A, got %s", m2.Name)
		}
	})
	t.Run("UpdateFields", func(t *testing.T) {
		s := newStore(t)
		m := &TestModel{ID: "m2", Name: "A"}
		s.Create(m)
		mUpdate := &TestModel{ID: "m2", Name: "B", CreatedAt: 999}
		if err := s.UpdateFields(mUpdate, "m2", []string{"name"}); err != nil {
			t.Fatal(err)
		}
		m2 := &TestModel{}
		s.ReadOne(m2, "m2")
		if m2.Name != "B" {
			t.Errorf("expected name B, got %s", m2.Name)
		}
		if m2.CreatedAt != 0 {
			t.Errorf("CreatedAt should not be updated")
		}
	})
	t.Run("ReadAllAndAfterLimit", func(t *testing.T) {
		s := newStore(t)
		s.Create(&TestModel{ID: "1"})
		s.Create(&TestModel{ID: "2"})
		s.Create(&TestModel{ID: "3"})
		q := crud.Query{Limit: 2, After: "1"}
		var res []string
		newRow := func() model.Model { return &TestModel{} }
		err := s.ReadAll(newRow, q, func(m model.Model) {
			res = append(res, m.(*TestModel).ID)
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(res) != 2 || res[0] != "2" || res[1] != "3" {
			t.Errorf("unexpected res %v", res)
		}
	})
	t.Run("ErrAfterWithOrder", func(t *testing.T) {
		s := newStore(t)
		q := crud.Query{After: "1", Order: []storage.Order{storage.Asc("name")}}
		newRow := func() model.Model { return &TestModel{} }
		err := s.ReadAll(newRow, q, func(m model.Model) {})
		if err != crud.ErrAfterWithOrder {
			t.Errorf("expected ErrAfterWithOrder, got %v", err)
		}
	})
	t.Run("OrCondition", func(t *testing.T) {
		s := newStore(t)
		q := crud.Query{Conditions: []storage.Condition{storage.Or(storage.Eq("name", "A"))}}
		newRow := func() model.Model { return &TestModel{} }
		err := s.ReadAll(newRow, q, func(m model.Model) {})
		if err != crud.ErrOrCondition {
			t.Errorf("expected ErrOrCondition, got %v", err)
		}
	})
	t.Run("AtomicRollback", func(t *testing.T) {
		s := newStore(t)
		txStore, ok := s.(crud.Transactional)
		if !ok {
			return
		}
			errFail := testErr("fail")
		err := txStore.Atomic(func(tx crud.Tx) error {
			tx.Create(&TestModel{ID: "4"})
			return errFail
		})
		if err != errFail {
			t.Errorf("expected errFail, got %v", err)
		}
		if err := s.ReadOne(&TestModel{}, "4"); model.KindOf(err) != model.ErrorNotFound {
			t.Errorf("expected not found (rolled back), got %v", err)
		}
	})
}

type testErr string

func (e testErr) Error() string { return string(e) }
