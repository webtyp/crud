package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/json"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/router/loopback"
)

func TestArgsJSON_ExactFormat(t *testing.T) {
	// 1. CreateArgs: {"records":[{"id":"1","name":"one"}]}
	rec := &mockRecord{ID: "1", Name: "one"}
	createArgs := &crud.CreateArgs{Records: []model.Model{rec}}
	var createBuf []byte
	if err := json.Encode(createArgs, &createBuf); err != nil {
		t.Fatalf("encode CreateArgs: %v", err)
	}
	wantCreateJSON := `{"records":[{"id":"1","name":"one"}]}`
	if string(createBuf) != wantCreateJSON {
		t.Errorf("CreateArgs JSON = %s, want %s", string(createBuf), wantCreateJSON)
	}

	// 2. UpdateArgs: {"ids":["1","2"],"fields":["name"],"record":{"id":"1","name":"one"}}
	updateArgs := &crud.UpdateArgs{
		IDs:    []string{"1", "2"},
		Fields: []string{"name"},
		Record: &mockRecord{ID: "1", Name: "one"},
	}
	var updateBuf []byte
	if err := json.Encode(updateArgs, &updateBuf); err != nil {
		t.Fatalf("encode UpdateArgs: %v", err)
	}
	wantUpdateJSON := `{"ids":["1","2"],"fields":["name"],"record":{"id":"1","name":"one"}}`
	if string(updateBuf) != wantUpdateJSON {
		t.Errorf("UpdateArgs JSON = %s, want %s", string(updateBuf), wantUpdateJSON)
	}

	// 3. DeleteArgs: {"ids":["1","2"]}
	deleteArgs := &crud.DeleteArgs{IDs: []string{"1", "2"}}
	var delBuf []byte
	if err := json.Encode(deleteArgs, &delBuf); err != nil {
		t.Fatalf("encode DeleteArgs: %v", err)
	}
	wantDelJSON := `{"ids":["1","2"]}`
	if string(delBuf) != wantDelJSON {
		t.Errorf("DeleteArgs JSON = %s, want %s", string(delBuf), wantDelJSON)
	}
}

type argsCodecModule struct {
	savedRecs     []*mockRecord
	patchedIDs    []string
	patchedFields []string
	patchedRec    mockRecord
	deletedIDs    []string
}

func (m *argsCodecModule) ModelName() string { return "argscodec" }
func (m *argsCodecModule) MountOperations(reg router.OperationRegistry) {
	crud.MountCreate(reg, func() model.ModelSlice { return &mockRecordList{} }, func(list model.ModelSlice) error {
		m.savedRecs = append(m.savedRecs, *list.(*mockRecordList)...)
		return nil
	})
	crud.MountUpdate(reg, func() model.Model { return &mockRecord{} }, func(ids, fields []string, rec model.Model) error {
		m.patchedIDs = append(m.patchedIDs, ids...)
		m.patchedFields = append(m.patchedFields, fields...)
		m.patchedRec = *rec.(*mockRecord)
		return nil
	})
	crud.MountDelete(reg, &mockRecord{}, func(ids []string) error {
		m.deletedIDs = append(m.deletedIDs, ids...)
		return nil
	})
}

func TestArgsJSON_LoopbackDecode(t *testing.T) {
	mod := &argsCodecModule{}
	c := loopback.ActingAs("tenant", "user", mod)

	// CreateArgs decoded through MountCreate (createIn)
	rec := &mockRecord{ID: "rec1", Name: "first"}
	var out mockRecord
	var err error
	c.Call("argscodec."+crud.Op(&mockRecord{}, model.Create), &crud.CreateArgs{Records: []model.Model{rec}}, &out, func(e error) { err = e })
	if err != nil {
		t.Fatalf("create call failed: %v", err)
	}
	if len(mod.savedRecs) != 1 || mod.savedRecs[0].ID != "rec1" || mod.savedRecs[0].Name != "first" {
		t.Fatalf("createIn did not decode CreateArgs properly: %+v", mod.savedRecs)
	}
	if out.ID != "rec1" || out.Name != "first" {
		t.Fatalf("create returned unexpected record: %+v", out)
	}

	// UpdateArgs decoded through MountUpdate (updateIn): 1 ID / 1 field (fast-path)
	c.Call("argscodec."+crud.Op(&mockRecord{}, model.Update), &crud.UpdateArgs{
		IDs:    []string{"id-1"},
		Fields: []string{"name"},
		Record: &mockRecord{ID: "id-1", Name: "updated"},
	}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("single update call failed: %v", err)
	}
	if len(mod.patchedIDs) != 1 || mod.patchedIDs[0] != "id-1" || len(mod.patchedFields) != 1 || mod.patchedFields[0] != "name" || mod.patchedRec.Name != "updated" {
		t.Fatalf("updateIn did not decode single UpdateArgs properly: ids=%v fields=%v rec=%+v", mod.patchedIDs, mod.patchedFields, mod.patchedRec)
	}

	// DeleteArgs decoded through MountDelete (deleteIn): 1 ID (fast-path) and multiple IDs
	c.Call("argscodec."+crud.Op(&mockRecord{}, model.Delete), &crud.DeleteArgs{IDs: []string{"id-single"}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("single delete call failed: %v", err)
	}
	c.Call("argscodec."+crud.Op(&mockRecord{}, model.Delete), &crud.DeleteArgs{IDs: []string{"id-a", "id-b"}}, nil, func(e error) { err = e })
	if err != nil {
		t.Fatalf("delete call failed: %v", err)
	}
	if len(mod.deletedIDs) != 3 || mod.deletedIDs[0] != "id-single" || mod.deletedIDs[1] != "id-a" || mod.deletedIDs[2] != "id-b" {
		t.Fatalf("deleteIn did not decode DeleteArgs properly: %+v", mod.deletedIDs)
	}
}
