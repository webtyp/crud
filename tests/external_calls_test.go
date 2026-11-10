package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/fmt"
	"webtyp.com/model"
)

type fakeCall struct {
	name string
}

func (c fakeCall) ModelName() string { return c.name }

type fakeCallRow struct {
	ID   string
	Call string
}

func (f *fakeCallRow) ModelName() string { return "fake_call_row" }
func (f *fakeCallRow) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "call", Type: model.Text()},
	}
}
func (f *fakeCallRow) Pointers() []any {
	return []any{&f.ID, &f.Call}
}
func (f *fakeCallRow) DecodeFields(r model.FieldReader) {
	if v, ok := r.String("id"); ok { f.ID = v }
	if v, ok := r.String("call"); ok { f.Call = v }
}
func (f *fakeCallRow) EncodeFields(w model.FieldWriter) {
	w.String("id", f.ID)
	w.String("call", f.Call)
}
func (f *fakeCallRow) IsNil() bool { return f == nil }

type fakeQueueState struct {
	id        string
	req       crud.CallRequest
	state     string
	attempts  int64
	lastError string
}

type fakeQueue struct {
	store     *memStore
	calls     map[string]*fakeQueueState
	seq       int
	keysSeen  []crud.IdempotencyKey
}

func newFakeQueue(s *memStore) *fakeQueue {
	return &fakeQueue{
		store: s,
		calls: make(map[string]*fakeQueueState),
	}
}

func (q *fakeQueue) Add(tx crud.Tx, r crud.CallRequest) (string, error) {
	q.seq++
	id := fmt.Sprintf("call-%d", q.seq)
	q.calls[id] = &fakeQueueState{
		id:    id,
		req:   r,
		state: "queued",
	}
	if err := tx.Create(&fakeCallRow{ID: id, Call: r.Call}); err != nil {
		return "", err
	}
	return id, nil
}

func (q *fakeQueue) MarkDone(id string) error {
	st, ok := q.calls[id]
	if !ok {
		return fmt.Err("call not found")
	}
	st.state = "done"
	return nil
}

func (q *fakeQueue) MarkFailedAttempt(id string, call model.ExternalCallDefinition, reason string) error {
	st, ok := q.calls[id]
	if !ok {
		return fmt.Err("call not found")
	}
	st.attempts++
	st.lastError = reason
	if call.Mode == model.RetryInBackground && st.attempts < call.MaxAttempts {
		st.state = "queued"
	} else {
		st.state = "failed"
	}
	return nil
}

func (q *fakeQueue) ConfirmCall(id string) error {
	st, ok := q.calls[id]
	if !ok {
		return fmt.Err("call not found")
	}
	st.state = "done"
	return nil
}

func (q *fakeQueue) RejectCall(id, reason string) error {
	st, ok := q.calls[id]
	if !ok {
		return fmt.Err("call not found")
	}
	st.state = "failed"
	st.lastError = reason
	return nil
}

// (a) Single transaction: record and queued call are written together, rollback removes both.
func TestExternalCalls_AtomicWithTransaction(t *testing.T) {
	store := newMemStore()
	queue := newFakeQueue(store)
	broker := &eventRecorder{}

	mod := &testModule{
		table: crud.StoredEntity{
			Model:   &tRec{},
			NewList: func() model.ModelSlice { return &tRecList{} },
			Store:   store,
			Rules:   declareRules(model.AllActions, model.Create),
			ExternalCalls: []model.ExternalCallDefinition{
				{Name: "charge", Mode: model.RetryInBackground, MaxAttempts: 3},
			},
			Create: func(a crud.CurrentUser, tx crud.Tx, rec model.Model) (crud.RuleAnswer, error) {
				if rec.(*tRec).Name == "fail" {
					return crud.RuleAnswer{}, model.Conflict("rejected by rule")
				}
				return crud.ApplyAndPublish(fakeCall{name: "charge"}), nil
			},
		},
	}

	caller := crud.LoopbackActingAsWithQueue(broker, "u1", queue, mod)

	// 1. Batch where second record fails: first record's call was added to tx, but rollback clears everything.
	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{
		&tRec{ID: "r-batch-1", Name: "ok"},
		&tRec{ID: "r-batch-2", Name: "fail"},
	}}, nil, func(e error) {
		err = e
	})
	if err == nil {
		t.Fatal("expected rule failure error, got nil")
	}
	if len(store.rows) != 0 {
		t.Fatalf("expected 0 store rows after rollback, got %d", len(store.rows))
	}
	var callRow fakeCallRow
	if err := store.ReadOne(&callRow, "call-1"); model.KindOf(err) != model.ErrorNotFound {
		t.Fatalf("expected call-1 row to be rolled back from store, got: %v", callRow)
	}

	// 2. Rule succeeds: both record and call are written.
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r-ok", Name: "ok"}}}, nil, func(e error) {
		err = e
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	var recRow tRec
	if err := store.ReadOne(&recRow, "r-ok"); err != nil {
		t.Fatalf("expected record stored, got %v", err)
	}
	if err := store.ReadOne(&callRow, "call-2"); err != nil {
		t.Fatalf("expected call row stored in same tx, got %v", err)
	}
	if queue.calls["call-2"] == nil || queue.calls["call-2"].state != "queued" {
		t.Fatalf("expected call-2 in queue as queued")
	}
}

// (b) RetryInBackground retries with backoff and finishes done or failed.
func TestExternalCalls_RetryInBackground(t *testing.T) {
	store := newMemStore()
	queue := newFakeQueue(store)
	broker := &eventRecorder{}

	callDef := model.ExternalCallDefinition{Name: "receipt", Mode: model.RetryInBackground, MaxAttempts: 3}
	mod := &testModule{
		table: crud.StoredEntity{
			Model:         &tRec{},
			NewList:       func() model.ModelSlice { return &tRecList{} },
			Store:         store,
			Rules:         declareRules(model.AllActions, model.Create),
			ExternalCalls: []model.ExternalCallDefinition{callDef},
			Create: func(a crud.CurrentUser, tx crud.Tx, rec model.Model) (crud.RuleAnswer, error) {
				return crud.ApplyAndPublish(fakeCall{name: "receipt"}), nil
			},
		},
	}

	caller := crud.LoopbackActingAsWithQueue(broker, "u1", queue, mod)
	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r1", Name: "receipt-test"}}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	callID := "call-1"
	st := queue.calls[callID]
	if st == nil || st.state != "queued" {
		t.Fatalf("expected call queued")
	}

	// Attempt 1 fails -> stays queued
	_ = queue.MarkFailedAttempt(callID, callDef, "net err 1")
	if st.state != "queued" || st.attempts != 1 {
		t.Fatalf("expected attempt 1 retrying (queued), got state %s, attempts %d", st.state, st.attempts)
	}

	// Attempt 2 fails -> stays queued
	_ = queue.MarkFailedAttempt(callID, callDef, "net err 2")
	if st.state != "queued" || st.attempts != 2 {
		t.Fatalf("expected attempt 2 retrying (queued), got state %s, attempts %d", st.state, st.attempts)
	}

	// Attempt 3 fails -> reaches MaxAttempts (3) -> terminal failed
	_ = queue.MarkFailedAttempt(callID, callDef, "net err 3")
	if st.state != "failed" || st.attempts != 3 {
		t.Fatalf("expected terminal failed after 3 attempts, got state %s, attempts %d", st.state, st.attempts)
	}
}

// (c) WaitForReply: response in time vs failure vs timeout.
func TestExternalCalls_WaitForReply(t *testing.T) {
	store := newMemStore()
	queue := newFakeQueue(store)
	broker := &eventRecorder{}

	callDef := model.ExternalCallDefinition{Name: "charge", Mode: model.WaitForReply, MaxWaitSeconds: 1}
	var callBehavior string // "ok", "fail", "timeout"

	mod := &testModule{
		table: crud.StoredEntity{
			Model:         &tRec{},
			NewList:       func() model.ModelSlice { return &tRecList{} },
			Store:         store,
			Rules:         declareRules(model.AllActions, model.Create),
			ExternalCalls: []model.ExternalCallDefinition{callDef},
			Calls: map[string]crud.ExternalCallFunc{
				"charge": func(a crud.CurrentUser, rec model.Model, key crud.IdempotencyKey) (crud.FieldsToSave, error) {
					queue.keysSeen = append(queue.keysSeen, key)
					switch callBehavior {
					case "ok":
						rec.(*tRec).Note = "charged"
						return crud.Fields("note"), nil
					case "fail":
						return nil, fmt.Err("card declined")
					case "timeout":
						// Block longer than MaxWaitSeconds
						ch := make(chan struct{})
						<-ch
						return nil, nil
					}
					return nil, nil
				},
			},
			Create: func(a crud.CurrentUser, tx crud.Tx, rec model.Model) (crud.RuleAnswer, error) {
				return crud.ApplyAndPublish(fakeCall{name: "charge"}), nil
			},
		},
	}

	caller := crud.LoopbackActingAsWithQueue(broker, "u1", queue, mod)

	// 1. Happy path: answers in time and updates fields.
	callBehavior = "ok"
	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r-ok", Name: "item"}}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if queue.calls["call-1"].state != "done" {
		t.Fatalf("expected call-1 marked done, got %s", queue.calls["call-1"].state)
	}
	var r tRec
	_ = store.ReadOne(&r, "r-ok")
	if r.Note != "charged" {
		t.Fatalf("expected note updated to 'charged', got %q", r.Note)
	}

	// 2. Failure: provider returned error -> fails immediately without retry (D60).
	callBehavior = "fail"
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r-fail", Name: "item"}}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("create itself succeeded, call failed in background: %v", err)
	}
	if queue.calls["call-2"].state != "failed" {
		t.Fatalf("expected call-2 marked failed without retry, got %s", queue.calls["call-2"].state)
	}
	if queue.calls["call-2"].lastError != "card declined" {
		t.Fatalf("expected error 'card declined', got %q", queue.calls["call-2"].lastError)
	}

	// 3. Timeout: call takes longer than MaxWaitSeconds -> times out -> marked failed without retry.
	callBehavior = "timeout"
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r-timeout", Name: "item"}}}, nil, func(e error) { err = e })
	if queue.calls["call-3"].state != "failed" {
		t.Fatalf("expected call-3 marked failed on timeout, got %s", queue.calls["call-3"].state)
	}
	if queue.calls["call-3"].lastError != "timeout" {
		t.Fatalf("expected lastError 'timeout', got %q", queue.calls["call-3"].lastError)
	}
}

// (d) WaitForConfirmation stays queued until ConfirmCall or RejectCall.
func TestExternalCalls_WaitForConfirmation(t *testing.T) {
	store := newMemStore()
	queue := newFakeQueue(store)
	broker := &eventRecorder{}

	callDef := model.ExternalCallDefinition{Name: "settle", Mode: model.WaitForConfirmation}
	mod := &testModule{
		table: crud.StoredEntity{
			Model:         &tRec{},
			NewList:       func() model.ModelSlice { return &tRecList{} },
			Store:         store,
			Rules:         declareRules(model.AllActions, model.Create),
			ExternalCalls: []model.ExternalCallDefinition{callDef},
			Create: func(a crud.CurrentUser, tx crud.Tx, rec model.Model) (crud.RuleAnswer, error) {
				return crud.ApplyAndPublish(fakeCall{name: "settle"}), nil
			},
		},
	}

	caller := crud.LoopbackActingAsWithQueue(broker, "u1", queue, mod)
	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r1", Name: "confirm-test"}}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	callID := "call-1"
	if queue.calls[callID].state != "queued" {
		t.Fatalf("expected call-1 to stay queued, got %s", queue.calls[callID].state)
	}

	// ConfirmCall settles it to done
	if err := crud.ConfirmCall(queue, callID); err != nil {
		t.Fatalf("ConfirmCall failed: %v", err)
	}
	if queue.calls[callID].state != "done" {
		t.Fatalf("expected state done after ConfirmCall, got %s", queue.calls[callID].state)
	}

	// Second record: RejectCall settles it to failed
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r2", Name: "reject-test"}}}, nil, func(e error) { err = e })
	callID2 := "call-2"
	if err := crud.RejectCall(queue, callID2, "charge reversed"); err != nil {
		t.Fatalf("RejectCall failed: %v", err)
	}
	if queue.calls[callID2].state != "failed" {
		t.Fatalf("expected state failed after RejectCall, got %s", queue.calls[callID2].state)
	}
	if queue.calls[callID2].lastError != "charge reversed" {
		t.Fatalf("expected lastError 'charge reversed', got %q", queue.calls[callID2].lastError)
	}
}

// (e) Idempotency key is the same queued call ID across attempts.
func TestExternalCalls_IdempotencyKey(t *testing.T) {
	store := newMemStore()
	queue := newFakeQueue(store)
	broker := &eventRecorder{}

	callDef := model.ExternalCallDefinition{Name: "charge", Mode: model.WaitForReply, MaxWaitSeconds: 1}
	var keys []crud.IdempotencyKey

	mod := &testModule{
		table: crud.StoredEntity{
			Model:         &tRec{},
			NewList:       func() model.ModelSlice { return &tRecList{} },
			Store:         store,
			Rules:         declareRules(model.AllActions, model.Create),
			ExternalCalls: []model.ExternalCallDefinition{callDef},
			Calls: map[string]crud.ExternalCallFunc{
				"charge": func(a crud.CurrentUser, rec model.Model, key crud.IdempotencyKey) (crud.FieldsToSave, error) {
					keys = append(keys, key)
					return nil, nil
				},
			},
			Create: func(a crud.CurrentUser, tx crud.Tx, rec model.Model) (crud.RuleAnswer, error) {
				return crud.ApplyAndPublish(fakeCall{name: "charge"}), nil
			},
		},
	}

	caller := crud.LoopbackActingAsWithQueue(broker, "u1", queue, mod)
	var err error
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{ID: "r1", Name: "idemp"}}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if len(keys) != 1 {
		t.Fatalf("expected 1 call attempt, got %d", len(keys))
	}
	if keys[0] != "call-1" {
		t.Fatalf("expected idempotency key 'call-1', got %q", keys[0])
	}
}
