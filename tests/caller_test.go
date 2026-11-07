package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/model"
)

type callerTestModule struct {
	lastUserID string
}

func (m *callerTestModule) ModelName() string { return "callertest" }
func (m *callerTestModule) MountOperations(reg crud.Registry) {
	reg.Create(func() model.ModelSlice { return &mockRecordList{} }, func(c crud.Caller, list model.ModelSlice) error {
		m.lastUserID = c.UserID
		return nil
	})
}

func TestCaller_UserIDFromSession(t *testing.T) {
	mod := &callerTestModule{}
	caller := crud.LoopbackActingAs("u1", mod)

	var out mockRecord
	var err error
	caller.Call("callertest."+crud.Op(&mockRecord{}, model.Create), &crud.CreateArgs{Records: []model.Model{&mockRecord{Name: "a"}}}, &out, func(e error) {
		err = e
	})
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	if mod.lastUserID != "u1" {
		t.Fatalf("expected caller UserID 'u1', got %q", mod.lastUserID)
	}
}

func TestCaller_AnonymousLoopback(t *testing.T) {
	mod := &callerTestModule{}
	caller := crud.Loopback(mod)

	var out mockRecord
	var err error
	caller.Call("callertest."+crud.Op(&mockRecord{}, model.Create), &crud.CreateArgs{Records: []model.Model{&mockRecord{Name: "a"}}}, &out, func(e error) {
		err = e
	})
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	if mod.lastUserID != "" {
		t.Fatalf("expected caller UserID empty string, got %q", mod.lastUserID)
	}
}
