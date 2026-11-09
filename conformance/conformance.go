package conformance

import (
	"sync"
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

func (m *TestModel) IsNil() bool                      { return m == nil }
func (m *TestModel) EncodeFields(w model.FieldWriter) {}
func (m *TestModel) DecodeFields(r model.FieldReader) {}
func (m *TestModel) Pointers() []any                  { return []any{&m.ID, &m.Name, &m.CreatedAt, &m.UpdatedAt} }
func (m *TestModel) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "name", Type: model.Text(), Item: model.ItemLabel},
		{Name: "created_at", Type: model.Int()},
		{Name: "updated_at", Type: model.Int()},
	}
}
func (m *TestModel) ModelName() string   { return "test_model" }
func (m *TestModel) Allow() model.Action { return model.AllActions }

// RevisionModel is the second table of the suite: a unique column and a revision counter, for the duplicate-key and conditional-update clauses.
type RevisionModel struct {
	ID       string
	Name     string
	Code     string
	Revision int64
}

func (m *RevisionModel) IsNil() bool                      { return m == nil }
func (m *RevisionModel) EncodeFields(w model.FieldWriter) {}
func (m *RevisionModel) DecodeFields(r model.FieldReader) {}
func (m *RevisionModel) Pointers() []any                  { return []any{&m.ID, &m.Name, &m.Code, &m.Revision} }
func (m *RevisionModel) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "name", Type: model.Text()},
		{Name: "code", Type: model.Text(), DB: &model.FieldDB{Unique: true}},
		{Name: "revision", Type: model.Int(), DB: &model.FieldDB{Revision: true}},
	}
}
func (m *RevisionModel) ModelName() string   { return "revision_model" }
func (m *RevisionModel) Allow() model.Action { return model.AllActions }

// Models lists the tables the suite writes. The factory of an implementation creates a table for each before returning the store
// (an implementation that creates tables on first write, like storage/mem, needs nothing).
func Models() []model.Model { return []model.Model{&TestModel{}, &RevisionModel{}} }

// writers is how many goroutines race in the concurrency clauses.
const writers = 8

// race runs fn from `writers` goroutines at the same time and returns what each returned.
func race(fn func(i int) error) []error {
	out := make([]error, writers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			out[i] = fn(i)
		}(i)
	}
	close(start)
	wg.Wait()
	return out
}

func winners(errs []error) (won, duplicates, other int) {
	for _, err := range errs {
		switch {
		case err == nil:
			won++
		case storage.IsDuplicate(err):
			duplicates++
		default:
			other++
		}
	}
	return
}

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
	t.Run("DuplicatePrimaryKeyIsReported", func(t *testing.T) {
		s := newStore(t)
		if err := s.Create(&TestModel{ID: "dup"}); err != nil {
			t.Fatal(err)
		}
		if err := s.Create(&TestModel{ID: "dup"}); !storage.IsDuplicate(err) {
			t.Errorf("a second Create of the same key must satisfy storage.IsDuplicate, got %v", err)
		}
	})
	t.Run("DuplicateUniqueColumnIsReported", func(t *testing.T) {
		s := newStore(t)
		if err := s.Create(&RevisionModel{ID: "u1", Code: "same"}); err != nil {
			t.Fatal(err)
		}
		if err := s.Create(&RevisionModel{ID: "u2", Code: "same"}); !storage.IsDuplicate(err) {
			t.Errorf("a repeated unique column must satisfy storage.IsDuplicate, got %v", err)
		}
	})
	t.Run("ConcurrentCreateOfOneKeyHasOneWinner", func(t *testing.T) {
		s := newStore(t)
		won, duplicates, other := winners(race(func(int) error { return s.Create(&TestModel{ID: "race"}) }))
		if won != 1 || duplicates != writers-1 || other != 0 {
			t.Errorf("%d writers on one key: %d won, %d duplicates, %d other errors; want 1, %d, 0", writers, won, duplicates, other, writers-1)
		}
	})
	t.Run("ConcurrentCreateOfOneUniqueValueHasOneWinner", func(t *testing.T) {
		s := newStore(t)
		ids := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
		won, duplicates, other := winners(race(func(i int) error { return s.Create(&RevisionModel{ID: ids[i], Code: "only"}) }))
		if won != 1 || duplicates != writers-1 || other != 0 {
			t.Errorf("%d writers on one unique value: %d won, %d duplicates, %d other errors; want 1, %d, 0", writers, won, duplicates, other, writers-1)
		}
	})
	t.Run("ConditionalUpdateAppliesOnlyWhileTheRevisionMatches", func(t *testing.T) {
		s := newStore(t)
		cu, ok := s.(crud.ConditionalUpdater)
		if !ok {
			t.Skip("the store is not a crud.ConditionalUpdater: it cannot serve an entity with a Revision field")
		}
		if err := s.Create(&RevisionModel{ID: "r1", Name: "first", Code: "r1", Revision: 1}); err != nil {
			t.Fatal(err)
		}
		rows, err := cu.UpdateFieldsIf(&RevisionModel{Name: "second", Revision: 2}, "r1", []string{"name", "revision"}, "revision", 1)
		if err != nil || rows != 1 {
			t.Fatalf("update from revision 1: rows %d, err %v; want 1, nil", rows, err)
		}
		rows, err = cu.UpdateFieldsIf(&RevisionModel{Name: "third", Revision: 2}, "r1", []string{"name", "revision"}, "revision", 1)
		if err != nil || rows != 0 {
			t.Fatalf("a stale update from revision 1: rows %d, err %v; want 0, nil", rows, err)
		}
		got := &RevisionModel{}
		if err := s.ReadOne(got, "r1"); err != nil || got.Name != "second" || got.Revision != 2 {
			t.Errorf("stored row = %+v (%v); want name second, revision 2", got, err)
		}
	})
	t.Run("ConcurrentConditionalUpdatesHaveOneWinner", func(t *testing.T) {
		s := newStore(t)
		cu, ok := s.(crud.ConditionalUpdater)
		if !ok {
			t.Skip("the store is not a crud.ConditionalUpdater")
		}
		if err := s.Create(&RevisionModel{ID: "c1", Code: "c1", Revision: 1}); err != nil {
			t.Fatal(err)
		}
		var mu sync.Mutex
		applied := int64(0)
		race(func(int) error {
			rows, err := cu.UpdateFieldsIf(&RevisionModel{Name: "x", Revision: 2}, "c1", []string{"name", "revision"}, "revision", 1)
			mu.Lock()
			applied += rows
			mu.Unlock()
			return err
		})
		if applied != 1 {
			t.Errorf("%d concurrent updates from the same revision applied %d times; want exactly 1", writers, applied)
		}
	})
}

type testErr string

func (e testErr) Error() string { return string(e) }
