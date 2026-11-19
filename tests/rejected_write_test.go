package tests

import (
	"testing"

	"webtyp.com/events/inprocess"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/securityevent"

	"webtyp.com/crud"
)

// TestWrite_RejectedInputIsASecurityEvent: a write that breaks the declared shape of the data is answered as before
// and reported as input_validation_fail, with who and the entity (SECURITY_OBSERVABILITY O3).
func TestWrite_RejectedInputIsASecurityEvent(t *testing.T) {
	broker := &inprocess.Broker{}
	var got []securityevent.Event
	if _, err := securityevent.Listen(broker, func(e securityevent.Event) { got = append(got, e) }); err != nil {
		t.Fatal(err)
	}
	mod := &testModule{table: crud.StoredEntity{
		Model:   &cpKeyRec{},
		NewList: func() model.ModelSlice { return &cpKeyList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, 0),
	}}
	caller := crud.LoopbackActingAs(broker, "u1", mod)
	var err error
	caller.Call(router.Operation{Module: "tmod", Entity: "cp", Action: model.Create}, &crud.CreateArgs{Records: []model.Model{&cpKeyRec{}}}, nil, func(e error) { err = e })
	if err == nil {
		t.Fatal("an empty required key must be refused")
	}
	if len(got) != 1 || got[0].Kind != securityevent.InputRejected || got[0].UserID != "u1" || got[0].Resource != "cp" {
		t.Fatalf("events = %+v, want one input_validation_fail by u1 on cp", got)
	}
}
