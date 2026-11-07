package tests

import (
	"strings"
	"testing"

	"webtyp.com/crud"
	"webtyp.com/model"
)

type secRecord struct {
	ID    string
	Title string
}

func (s *secRecord) ModelName() string { return "sec_item" }
func (s *secRecord) IsNil() bool       { return s == nil }

func (s *secRecord) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text()},
		{Name: "title", Type: model.Text(), Permitted: model.Permitted{Maximum: 10}},
	}
}

func (s *secRecord) Pointers() []any {
	return []any{&s.ID, &s.Title}
}

func (s *secRecord) EncodeFields(w model.FieldWriter) {
	w.String("id", s.ID)
	w.String("title", s.Title)
}

func (s *secRecord) DecodeFields(r model.FieldReader) {
	if val, ok := r.String("id"); ok {
		s.ID = val
	}
	if val, ok := r.String("title"); ok {
		s.Title = val
	}
}

type secRecordList []*secRecord

func (s *secRecordList) Len() int                         { return len(*s) }
func (s *secRecordList) At(i int) model.Fielder           { return (*s)[i] }
func (s *secRecordList) Append() model.Fielder            { v := &secRecord{}; *s = append(*s, v); return v }
func (s *secRecordList) IsNil() bool                      { return s == nil }
func (s *secRecordList) EncodeFields(_ model.FieldWriter) {}
func (s *secRecordList) DecodeFields(_ model.FieldReader) {}

type secModule struct {
	created  []*secRecord
	lastRead crud.ReadArgs
	patched  func(ids, fields []string, rec secRecord) error
	deleted  []string
}

func (m *secModule) ModelName() string { return "secmod" }
func (m *secModule) MountOperations(reg crud.Registry) {
	reg.Create(func() model.ModelSlice { return &secRecordList{} }, func(c crud.Caller, list model.ModelSlice) error {
		for _, r := range *list.(*secRecordList) {
			m.created = append(m.created, r)
		}
		return nil
	})
	reg.Read(&secRecord{}, nil, func(c crud.Caller, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		m.lastRead = args
		out := secRecordList{}
		return &out, nil
	})
	reg.Update(func() model.Model { return &secRecord{} }, func(c crud.Caller, ids, fields []string, rec model.Model) error {
		if m.patched != nil {
			return m.patched(ids, fields, *rec.(*secRecord))
		}
		return nil
	})
	reg.Delete(&secRecord{}, func(c crud.Caller, ids []string) error {
		m.deleted = append(m.deleted, ids...)
		return nil
	})
}

func secOp(action model.Action) string {
	return "secmod." + crud.Op(&secRecord{}, action)
}

func TestSecurity_BatchLimitExceeded(t *testing.T) {
	mod := &secModule{}
	c := crud.LoopbackActingAs("u", mod)

	// 1. Create with 501 records (> MaxPage)
	largeBatch := make([]model.Model, crud.MaxPage+1)
	for i := range largeBatch {
		largeBatch[i] = &secRecord{ID: "id", Title: "ok"}
	}
	var err error
	c.Call(secOp(model.Create), &crud.CreateArgs{Records: largeBatch}, nil, func(e error) { err = e })
	if err == nil || !strings.Contains(err.Error(), string(crud.ErrBatchTooLarge)) {
		t.Fatalf("expected ErrBatchTooLarge on Create, got %v", err)
	}

	// 2. Update with 501 IDs (> MaxPage)
	largeIDs := make([]string, crud.MaxPage+1)
	for i := range largeIDs {
		largeIDs[i] = "item"
	}
	err = nil
	c.Call(secOp(model.Update), &crud.UpdateArgs{IDs: largeIDs, Fields: []string{"title"}, Record: &secRecord{Title: "new"}}, nil, func(e error) { err = e })
	if err == nil || !strings.Contains(err.Error(), string(crud.ErrBatchTooLarge)) {
		t.Fatalf("expected ErrBatchTooLarge on Update, got %v", err)
	}

	// 3. Delete with 501 IDs (> MaxPage)
	err = nil
	c.Call(secOp(model.Delete), &crud.DeleteArgs{IDs: largeIDs}, nil, func(e error) { err = e })
	if err == nil || !strings.Contains(err.Error(), string(crud.ErrBatchTooLarge)) {
		t.Fatalf("expected ErrBatchTooLarge on Delete, got %v", err)
	}
}

func TestSecurity_ImmutableAndUnknownFields(t *testing.T) {
	mod := &secModule{}
	c := crud.LoopbackActingAs("u", mod)

	// 1. Reject mutating "id"
	var err error
	c.Call(secOp(model.Update), &crud.UpdateArgs{
		IDs:    []string{"1"},
		Fields: []string{"id"},
		Record: &secRecord{ID: "evil"},
	}, nil, func(e error) { err = e })
	if err == nil || !strings.Contains(err.Error(), string(crud.ErrInvalidField)) {
		t.Fatalf("expected ErrInvalidField for id mutation, got %v", err)
	}

	// 2. Reject mutating "tenant_id"
	err = nil
	c.Call(secOp(model.Update), &crud.UpdateArgs{
		IDs:    []string{"1"},
		Fields: []string{"tenant_id"},
		Record: &secRecord{Title: "evil"},
	}, nil, func(e error) { err = e })
	if err == nil || !strings.Contains(err.Error(), string(crud.ErrInvalidField)) {
		t.Fatalf("expected ErrInvalidField for tenant_id mutation, got %v", err)
	}

	// 3. Reject unknown field
	err = nil
	c.Call(secOp(model.Update), &crud.UpdateArgs{
		IDs:    []string{"1"},
		Fields: []string{"nonexistent_column"},
		Record: &secRecord{Title: "x"},
	}, nil, func(e error) { err = e })
	if err == nil || !strings.Contains(err.Error(), string(crud.ErrUnknownField)) {
		t.Fatalf("expected ErrUnknownField, got %v", err)
	}

	// 4. Reject duplicate fields
	err = nil
	c.Call(secOp(model.Update), &crud.UpdateArgs{
		IDs:    []string{"1"},
		Fields: []string{"title", "title"},
		Record: &secRecord{Title: "x"},
	}, nil, func(e error) { err = e })
	if err == nil || model.KindOf(err) != model.ErrorInvalid {
		t.Fatalf("expected Invalid error on duplicate fields, got %v", err)
	}
}

func TestSecurity_InvalidOrEmptyIDs(t *testing.T) {
	mod := &secModule{}
	c := crud.LoopbackActingAs("u", mod)

	// 1. Update with empty ID
	var err error
	c.Call(secOp(model.Update), &crud.UpdateArgs{
		IDs:    []string{""},
		Fields: []string{"title"},
		Record: &secRecord{Title: "x"},
	}, nil, func(e error) { err = e })
	if err == nil || !strings.Contains(err.Error(), string(crud.ErrInvalidID)) {
		t.Fatalf("expected ErrInvalidID on empty string, got %v", err)
	}

	// 2. Update with whitespace-only ID
	err = nil
	c.Call(secOp(model.Update), &crud.UpdateArgs{
		IDs:    []string{"   "},
		Fields: []string{"title"},
		Record: &secRecord{Title: "x"},
	}, nil, func(e error) { err = e })
	if err == nil || !strings.Contains(err.Error(), string(crud.ErrInvalidID)) {
		t.Fatalf("expected ErrInvalidID on whitespace id, got %v", err)
	}

	// 3. Delete with empty ID
	err = nil
	c.Call(secOp(model.Delete), &crud.DeleteArgs{
		IDs: []string{""},
	}, nil, func(e error) { err = e })
	if err == nil || !strings.Contains(err.Error(), string(crud.ErrInvalidID)) {
		t.Fatalf("expected ErrInvalidID on Delete empty id, got %v", err)
	}

	// 4. Delete with whitespace ID
	err = nil
	c.Call(secOp(model.Delete), &crud.DeleteArgs{
		IDs: []string{" \t "},
	}, nil, func(e error) { err = e })
	if err == nil || !strings.Contains(err.Error(), string(crud.ErrInvalidID)) {
		t.Fatalf("expected ErrInvalidID on Delete whitespace id, got %v", err)
	}
}

func TestSecurity_CreateSchemaValidation(t *testing.T) {
	mod := &secModule{}
	c := crud.LoopbackActingAs("u", mod)

	// secRecord Schema specifies title Maximum: 10
	var err error
	c.Call(secOp(model.Create), &crud.CreateArgs{
		Records: []model.Model{
			&secRecord{ID: "1", Title: "exceeds_maximum_permitted_length"},
		},
	}, nil, func(e error) { err = e })

	if err == nil || model.KindOf(err) != model.ErrorInvalid {
		t.Fatalf("expected validation error on title exceeding maximum length, got %v", err)
	}
}

func TestSecurity_SearchTermCapped(t *testing.T) {
	mod := &secModule{}
	c := crud.LoopbackActingAs("u", mod)

	longSearch := strings.Repeat("a", 257)
	var err error
	c.Call(secOp(model.Read), &crud.ReadArgs{
		Search: longSearch,
		Limit:  1000,
	}, nil, func(e error) { err = e })

	if err == nil {
		t.Fatalf("expected error on read with search > 256 bytes")
	}
	if model.KindOf(err) != model.ErrorInvalid || !strings.Contains(err.Error(), "argument too long") {
		t.Fatalf("expected invalid argument too long, got %v", err)
	}
}

func TestSecurity_DuplicateIDs_Allowed(t *testing.T) {
	mod := &secModule{}
	c := crud.LoopbackActingAs("u", mod)

	var err error
	c.Call(secOp(model.Delete), &crud.DeleteArgs{
		IDs: []string{"id-1", "id-2", "id-1"},
	}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("expected duplicate IDs in Delete to be accepted without error, got %v", err)
	}
}

func TestSecurity_UpdateValueValidation(t *testing.T) {
	mod := &secModule{}
	c := crud.LoopbackActingAs("u", mod)

	// Update with title violating Permitted Maximum: 10
	var err error
	c.Call(secOp(model.Update), &crud.UpdateArgs{
		IDs:    []string{"1"},
		Fields: []string{"title"},
		Record: &secRecord{Title: "longer_than_ten_chars"},
	}, nil, func(e error) { err = e })
	if err == nil || model.KindOf(err) != model.ErrorInvalid {
		t.Fatalf("expected Invalid error on updated field exceeding Maximum, got %v", err)
	}
}

func TestSecurity_CursorAndIDCapped(t *testing.T) {
	mod := &secModule{}
	c := crud.LoopbackActingAs("u", mod)

	longID := strings.Repeat("y", 129)
	var err error
	c.Call(secOp(model.Read), &crud.ReadArgs{
		ID: longID,
	}, nil, func(e error) { err = e })

	if err == nil {
		t.Fatalf("expected error on read with ID > 128 bytes")
	}
	if model.KindOf(err) != model.ErrorInvalid || !strings.Contains(err.Error(), string(crud.ErrInvalidID)) {
		t.Fatalf("expected ErrInvalidID, got %v", err)
	}

	longAfter := strings.Repeat("x", 129)
	err = nil
	c.Call(secOp(model.Read), &crud.ReadArgs{
		After: longAfter,
	}, nil, func(e error) { err = e })

	if err == nil {
		t.Fatalf("expected error on read with After > 128 bytes")
	}
	if model.KindOf(err) != model.ErrorInvalid || !strings.Contains(err.Error(), "argument too long") {
		t.Fatalf("expected invalid argument too long, got %v", err)
	}
}
