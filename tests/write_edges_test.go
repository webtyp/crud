package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/router"
)

var errStoreDown = fmt.Err("store down")

// failingStore is the memory store whose transaction fails on the chosen write.
type failingStore struct {
	*memStore
	failOn string // "create", "update" or "delete"
}

func (s *failingStore) Atomic(fn func(crud.Tx) error) error {
	return s.memStore.Atomic(func(tx crud.Tx) error { return fn(&failingTx{Tx: tx, failOn: s.failOn}) })
}

func (s *failingStore) Create(rec model.Model) error {
	if s.failOn == "create" {
		return errStoreDown
	}
	return s.memStore.Create(rec)
}

func (s *failingStore) UpdateFields(rec model.Model, id string, fields []string) error {
	if s.failOn == "update" {
		return errStoreDown
	}
	return s.memStore.UpdateFields(rec, id, fields)
}

func (s *failingStore) Delete(rec model.Model, id string) error {
	if s.failOn == "delete" {
		return errStoreDown
	}
	return s.memStore.Delete(rec, id)
}

type failingTx struct {
	crud.Tx
	failOn string
}

func (t *failingTx) Create(rec model.Model) error {
	if t.failOn == "create" {
		return errStoreDown
	}
	return t.Tx.Create(rec)
}
func (t *failingTx) UpdateFields(rec model.Model, id string, fields []string) error {
	if t.failOn == "update" {
		return errStoreDown
	}
	return t.Tx.UpdateFields(rec, id, fields)
}
func (t *failingTx) Delete(rec model.Model, id string) error {
	if t.failOn == "delete" {
		return errStoreDown
	}
	return t.Tx.Delete(rec, id)
}

// unreadableArgs encodes an invalid JSON field before the target key, so any handler's decoder refuses the body.
type unreadableArgs struct{ key string }

func (a unreadableArgs) IsNil() bool { return false }
func (a unreadableArgs) EncodeFields(w model.FieldWriter) {
	w.Raw("corrupt", "???")
	w.String(a.key, "not what the decoder expects")
}

func TestWrite_UnreadableBodyIsRefused(t *testing.T) {
	store := newMemStore()
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)

	ops := []struct {
		op  router.Operation
		arg unreadableArgs
	}{
		{op: operationOf(model.Create), arg: unreadableArgs{key: "records"}},
		{op: operationOf(model.Update), arg: unreadableArgs{key: "ids"}},
		{op: operationOf(model.Delete), arg: unreadableArgs{key: "ids"}},
	}

	for _, tc := range ops {
		var err error
		caller.Call(tc.op, tc.arg, nil, func(e error) { err = e })
		if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrCannotRead) {
			t.Errorf("%v: expected ErrCannotRead, got %v", tc.op.Action, err)
		}
	}
	if len(store.rows) != 0 {
		t.Errorf("expected 0 rows in store, got %d", len(store.rows))
	}
}

func TestWrite_CreateTwiceWithTheSameIDDoesNotDuplicate(t *testing.T) {
	store := newMemStore()
	rec := &eventRecorder{}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(rec, "u1", mod)

	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "fixed", Name: "a"}}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	var ans tRec
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "fixed", Name: "b"}}}, &ans, func(e error) { err = e })
	if err != nil {
		t.Fatalf("second create failed: %v", err)
	}
	if ans.Name != "a" {
		t.Errorf("expected answered record Name 'a', got %q", ans.Name)
	}
	if len(store.rows) != 1 {
		t.Errorf("expected 1 row in store, got %d", len(store.rows))
	}
	if len(rec.got) != 1 {
		t.Errorf("expected exactly 1 event published, got %d", len(rec.got))
	}
}

func TestWrite_CompositeKeyEmptyIsRequired(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &cpKeyRec{},
		NewList: func() model.ModelSlice { return &cpKeyList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)
	var err error
	caller.Call(router.Operation{Module: "tmod", Entity: "cp", Action: model.Create}, &crud.CreateArgs{Records: []model.Model{&cpKeyRec{}}}, nil, func(e error) { err = e })
	if err == nil || !fmt.Contains(err.Error(), "required") || !fmt.Contains(err.Error(), "room_id") {
		t.Errorf("expected required and room_id in error, got %v", err)
	}
}

func TestWrite_StoreFailureRollsBackAndPublishesNothing(t *testing.T) {
	// Create failure
	{
		fs := &failingStore{memStore: newMemStore(), failOn: "create"}
		rec := &eventRecorder{}
		mod := &testModule{table: crud.StoredEntity{
			Model:   &tRec{},
			NewList: func() model.ModelSlice { return &tRecList{} },
			Store:   fs,
			Rules:   declareRules(model.AllActions, model.Create),
			Create: func(c crud.CurrentUser, tx crud.Tx, m model.Model) (crud.RuleAnswer, error) {
				return crud.ApplyAndPublish, nil
			},
		}}
		caller := crud.LoopbackActingAs(rec, "u1", mod)
		var err error
		caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{Name: "1"}, &tRec{Name: "2"}}}, nil, func(e error) { err = e })
		if err == nil || (!fmt.Contains(err.Error(), "store down") && !fmt.Contains(err.Error(), "500")) {
			t.Errorf("expected store down error on create, got %v", err)
		}
		if len(fs.rows) != 0 {
			t.Errorf("expected 0 rows after create failure, got %d", len(fs.rows))
		}
		if len(rec.got) != 0 {
			t.Errorf("expected 0 events, got %d", len(rec.got))
		}
	}

	// Update failure
	{
		fs := &failingStore{memStore: newMemStore(), failOn: "update"}
		fs.Create(&tRec{ID: "r1", Name: "orig"})
		rec := &eventRecorder{}
		mod := &testModule{table: crud.StoredEntity{
			Model:   &tRec{},
			NewList: func() model.ModelSlice { return &tRecList{} },
			Store:   fs,
			Rules:   declareRules(model.AllActions, model.Update),
			Update: func(c crud.CurrentUser, tx crud.Tx, old, next model.Model, fields []string) (crud.RuleAnswer, error) {
				return crud.ApplyAndPublish, nil
			},
		}}
		caller := crud.LoopbackActingAs(rec, "u1", mod)
		var err error
		caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: []string{"r1"}, Fields: []string{"name"}, Record: &tRec{Name: "new"}}, nil, func(e error) { err = e })
		if err == nil || (!fmt.Contains(err.Error(), "store down") && !fmt.Contains(err.Error(), "500")) {
			t.Errorf("expected store down error on update, got %v", err)
		}
		var stored tRec
		fs.ReadOne(&stored, "r1")
		if stored.Name != "orig" {
			t.Errorf("expected row to remain unchanged, got %q", stored.Name)
		}
		if len(rec.got) != 0 {
			t.Errorf("expected 0 events, got %d", len(rec.got))
		}
	}

	// Delete failure
	{
		fs := &failingStore{memStore: newMemStore(), failOn: "delete"}
		fs.Create(&tRec{ID: "r1", Name: "orig"})
		rec := &eventRecorder{}
		mod := &testModule{table: crud.StoredEntity{
			Model:   &tRec{},
			NewList: func() model.ModelSlice { return &tRecList{} },
			Store:   fs,
			Rules:   declareRules(model.AllActions, model.Delete),
			Delete: func(c crud.CurrentUser, tx crud.Tx, old model.Model) (crud.RuleAnswer, error) {
				return crud.ApplyAndPublish, nil
			},
		}}
		caller := crud.LoopbackActingAs(rec, "u1", mod)
		var err error
		caller.Call(operationOf(model.Delete), &crud.DeleteArgs{IDs: []string{"r1"}}, nil, func(e error) { err = e })
		if err == nil || (!fmt.Contains(err.Error(), "store down") && !fmt.Contains(err.Error(), "500")) {
			t.Errorf("expected store down error on delete, got %v", err)
		}
		if len(fs.rows) != 1 {
			t.Errorf("expected row to still be in store, got %d rows", len(fs.rows))
		}
		if len(rec.got) != 0 {
			t.Errorf("expected 0 events, got %d", len(rec.got))
		}
	}
}

func TestWrite_UpdateIDsAreValidated(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)

	// IDs: nil -> ErrNoIDs
	var err error
	caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: nil, Fields: []string{"name"}, Record: &tRec{Name: "v"}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrNoIDs) {
		t.Errorf("expected ErrNoIDs, got %v", err)
	}

	// IDs: {""} -> ErrInvalidID
	caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: []string{""}, Fields: []string{"name"}, Record: &tRec{Name: "v"}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrInvalidID) {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}

	// crud.MaxPage+1 ids -> ErrBatchTooLarge
	tooMany := make([]string, crud.MaxPage+1)
	for i := range tooMany {
		tooMany[i] = "id"
	}
	caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: tooMany, Fields: []string{"name"}, Record: &tRec{Name: "v"}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrBatchTooLarge) {
		t.Errorf("expected ErrBatchTooLarge, got %v", err)
	}
}

func TestWrite_DeleteIDsAreValidated(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)

	// IDs: nil -> ErrNoIDs
	var err error
	caller.Call(operationOf(model.Delete), &crud.DeleteArgs{IDs: nil}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrNoIDs) {
		t.Errorf("expected ErrNoIDs, got %v", err)
	}

	// IDs: {""} -> ErrInvalidID
	caller.Call(operationOf(model.Delete), &crud.DeleteArgs{IDs: []string{""}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrInvalidID) {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}

	// crud.MaxPage+1 ids -> ErrBatchTooLarge
	tooMany := make([]string, crud.MaxPage+1)
	for i := range tooMany {
		tooMany[i] = "id"
	}
	caller.Call(operationOf(model.Delete), &crud.DeleteArgs{IDs: tooMany}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrBatchTooLarge) {
		t.Errorf("expected ErrBatchTooLarge, got %v", err)
	}
}

func TestWrite_UpdateFieldsAreValidated(t *testing.T) {
	store := newMemStore()
	store.Create(&tRec{ID: "r1", Name: "orig"})
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)

	cases := []struct {
		fields []string
		expect model.Invalid
	}{
		{fields: nil, expect: crud.ErrNoFields},
		{fields: []string{"id"}, expect: crud.ErrInvalidField},
		{fields: []string{"name", "name"}, expect: crud.ErrInvalidField},
		{fields: []string{"nope"}, expect: crud.ErrUnknownField},
	}

	for _, tc := range cases {
		var err error
		caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: []string{"r1"}, Fields: tc.fields, Record: &tRec{Name: "val"}}, nil, func(e error) { err = e })
		if err == nil || err.Error() != "loopback: 400 "+string(tc.expect) {
			t.Errorf("fields %v: expected %s, got %v", tc.fields, tc.expect, err)
		}
	}
}

func TestWrite_BatchOnNonTransactionalStoreRejectedForUpdateAndDelete(t *testing.T) {
	type nonTxStore struct{ crud.Store }

	store := newMemStore()
	store.Create(&tRec{ID: "r1", Name: "a"})
	store.Create(&tRec{ID: "r2", Name: "b"})

	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   &nonTxStore{Store: store},
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)

	// Update with 2 ids -> ErrBatchNotAtomic
	var err error
	caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: []string{"r1", "r2"}, Fields: []string{"name"}, Record: &tRec{Name: "c"}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrBatchNotAtomic) {
		t.Errorf("update 2 ids: expected ErrBatchNotAtomic, got %v", err)
	}

	// Update with 1 id -> no error
	caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: []string{"r1"}, Fields: []string{"name"}, Record: &tRec{Name: "c"}}, nil, func(e error) { err = e })
	if err != nil {
		t.Errorf("update 1 id: expected no error, got %v", err)
	}

	// Delete with 2 ids -> ErrBatchNotAtomic
	caller.Call(operationOf(model.Delete), &crud.DeleteArgs{IDs: []string{"r1", "r2"}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrBatchNotAtomic) {
		t.Errorf("delete 2 ids: expected ErrBatchNotAtomic, got %v", err)
	}

	// Delete with 1 id -> no error
	caller.Call(operationOf(model.Delete), &crud.DeleteArgs{IDs: []string{"r1"}}, nil, func(e error) { err = e })
	if err != nil {
		t.Errorf("delete 1 id: expected no error, got %v", err)
	}
}

func TestWrite_IDWithSeparatorIsRejectedOnUpdateAndDelete(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)

	var err error
	caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: []string{"a/b"}, Fields: []string{"name"}, Record: &tRec{Name: "x"}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrKeyHasSeparator) {
		t.Errorf("update: expected ErrKeyHasSeparator, got %v", err)
	}

	caller.Call(operationOf(model.Delete), &crud.DeleteArgs{IDs: []string{"a/b"}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 400 "+string(crud.ErrKeyHasSeparator) {
		t.Errorf("delete: expected ErrKeyHasSeparator, got %v", err)
	}
}

func TestWrite_ZeroAnswerIsRefusedOnUpdateAndDelete(t *testing.T) {
	store := newMemStore()
	store.Create(&tRec{ID: "r1", Name: "orig"})

	rec := &eventRecorder{}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   store,
		Rules:   declareRules(model.AllActions, model.Update|model.Delete),
		Update: func(c crud.CurrentUser, tx crud.Tx, old, next model.Model, fields []string) (crud.RuleAnswer, error) {
			return 0, nil
		},
		Delete: func(c crud.CurrentUser, tx crud.Tx, old model.Model) (crud.RuleAnswer, error) {
			return 0, nil
		},
	}}
	caller := crud.LoopbackActingAs(rec, "u1", mod)

	// Update with zero answer
	var err error
	caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: []string{"r1"}, Fields: []string{"name"}, Record: &tRec{Name: "new"}}, nil, func(e error) { err = e })
	if err == nil || (!fmt.Contains(err.Error(), "500") && !fmt.Contains(err.Error(), "answered nothing")) {
		t.Errorf("update: expected 500 / ErrNoAnswer, got %v", err)
	}
	var row tRec
	store.ReadOne(&row, "r1")
	if row.Name != "orig" {
		t.Errorf("update: expected row to remain unchanged, got %q", row.Name)
	}
	if len(rec.got) != 0 {
		t.Errorf("update: expected 0 events, got %d", len(rec.got))
	}

	// Delete with zero answer
	caller.Call(operationOf(model.Delete), &crud.DeleteArgs{IDs: []string{"r1"}}, nil, func(e error) { err = e })
	if err == nil || (!fmt.Contains(err.Error(), "500") && !fmt.Contains(err.Error(), "answered nothing")) {
		t.Errorf("delete: expected 500 / ErrNoAnswer, got %v", err)
	}
	if len(store.rows) != 1 {
		t.Errorf("delete: expected row to still be in store, got %d rows", len(store.rows))
	}
	if len(rec.got) != 0 {
		t.Errorf("delete: expected 0 events, got %d", len(rec.got))
	}
}

func TestWrite_DeleteMissingIsNotFound(t *testing.T) {
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)
	var err error
	caller.Call(operationOf(model.Delete), &crud.DeleteArgs{IDs: []string{"nope"}}, nil, func(e error) { err = e })
	if err == nil || err.Error() != "loopback: 404 not found" {
		t.Errorf("expected 404 not found, got %v", err)
	}
}

// countingStore counts the transactions opened through Atomic.
type countingStore struct {
	*memStore
	atomics int
}

func (s *countingStore) Atomic(fn func(crud.Tx) error) error {
	s.atomics++
	return s.memStore.Atomic(fn)
}

func TestWrite_OneRecordWithoutRuleOpensNoTransaction(t *testing.T) {
	cs := &countingStore{memStore: newMemStore()}
	rec := &eventRecorder{}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   cs,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(rec, "u1", mod)

	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r1", Name: "orig"}}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: []string{"r1"}, Fields: []string{"name"}, Record: &tRec{Name: "mod"}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	caller.Call(operationOf(model.Delete), &crud.DeleteArgs{IDs: []string{"r1"}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if cs.atomics != 0 {
		t.Errorf("expected 0 atomics, got %d", cs.atomics)
	}
	if len(rec.got) != 3 {
		t.Errorf("expected 3 events, got %d", len(rec.got))
	}
}

func TestWrite_OneRecordWithRuleOpensOneTransaction(t *testing.T) {
	cs := &countingStore{memStore: newMemStore()}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   cs,
		Rules:   declareRules(model.AllActions, model.Create|model.Update|model.Delete),
		Create: func(c crud.CurrentUser, tx crud.Tx, m model.Model) (crud.RuleAnswer, error) {
			return crud.ApplyAndPublish, nil
		},
		Update: func(c crud.CurrentUser, tx crud.Tx, old, next model.Model, fields []string) (crud.RuleAnswer, error) {
			return crud.ApplyAndPublish, nil
		},
		Delete: func(c crud.CurrentUser, tx crud.Tx, old model.Model) (crud.RuleAnswer, error) {
			return crud.ApplyAndPublish, nil
		},
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)

	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r1", Name: "orig"}}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: []string{"r1"}, Fields: []string{"name"}, Record: &tRec{Name: "mod"}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	caller.Call(operationOf(model.Delete), &crud.DeleteArgs{IDs: []string{"r1"}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if cs.atomics != 3 {
		t.Errorf("expected 3 atomics, got %d", cs.atomics)
	}
}

func TestWrite_BatchOpensOneTransaction(t *testing.T) {
	cs := &countingStore{memStore: newMemStore()}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   cs,
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(nil, "u1", mod)

	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{Name: "1"}, &tRec{Name: "2"}, &tRec{Name: "3"}}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("create batch failed: %v", err)
	}

	if cs.atomics != 1 {
		t.Errorf("expected 1 atomic for batch, got %d", cs.atomics)
	}
}

func TestWrite_SimpleWriteFailureLeavesNothing(t *testing.T) {
	type nonTxStore struct{ crud.Store }
	fs := &failingStore{memStore: newMemStore(), failOn: "create"}
	rec := &eventRecorder{}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   &nonTxStore{Store: fs},
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(rec, "u1", mod)

	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{Name: "1"}}}, nil, func(e error) { err = e })
	if err == nil || (!fmt.Contains(err.Error(), "store down") && !fmt.Contains(err.Error(), "500")) {
		t.Errorf("expected store error, got %v", err)
	}
	if len(fs.rows) != 0 {
		t.Errorf("expected 0 rows in store, got %d", len(fs.rows))
	}
	if len(rec.got) != 0 {
		t.Errorf("expected 0 events, got %d", len(rec.got))
	}
}
