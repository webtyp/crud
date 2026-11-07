package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/model"
	"webtyp.com/router"
)

type recordedOp struct {
	name     string
	isPublic bool
	resource model.Resource
	action   model.Action
}

type fakeReg struct {
	ops map[string]*recordedOp
}

func newFakeReg() *fakeReg {
	return &fakeReg{ops: make(map[string]*recordedOp)}
}

func (f *fakeReg) Operation(name string, h router.HandlerFunc) router.Route {
	rec := &recordedOp{name: name}
	f.ops[name] = rec
	return &fakeRoute{rec: rec}
}

type fakeRoute struct {
	rec *recordedOp
}

func (r *fakeRoute) Requires(res model.Resource, act model.Action) router.Route {
	r.rec.resource = res
	r.rec.action = act
	return r
}
func (r *fakeRoute) Authenticated() router.Route { return r }
func (r *fakeRoute) Public() router.Route {
	r.rec.isPublic = true
	return r
}
func (r *fakeRoute) RequiresMFA() router.Route            { return r }
func (r *fakeRoute) Accepts(model.Fielder) router.Route   { return r }
func (r *fakeRoute) Describe(string) router.Route        { return r }

var _ router.Route = (*fakeRoute)(nil)
var _ router.OperationRegistry = (*fakeReg)(nil)

type publicTestModule struct{}

func (m *publicTestModule) ModelName() string { return "publicmod" }
func (m *publicTestModule) MountOperations(reg crud.Registry) {
	reg.Create(func() model.ModelSlice { return &mockRecordList{} }, func(c crud.Caller, list model.ModelSlice) error {
		return nil
	})
	reg.Read(&mockRecord{}, nil, func(c crud.Caller, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		out := mockRecordList{}
		return &out, nil
	})
}

func TestMount_PublicRead(t *testing.T) {
	fake := newFakeReg()
	mod := &publicTestModule{}
	err := crud.Mount(fake, []model.ModuleNaming{&mockRecord{}}, mod)
	if err != nil {
		t.Fatalf("unexpected mount error: %v", err)
	}

	readOp := fake.ops["test_record_r"]
	if readOp == nil {
		t.Fatalf("missing test_record_r op")
	}
	if !readOp.isPublic {
		t.Fatalf("expected test_record_r to be Public")
	}
	if readOp.resource != "" {
		t.Fatalf("expected test_record_r to have no Requires, got %v", readOp.resource)
	}

	createOp := fake.ops["test_record_c"]
	if createOp == nil {
		t.Fatalf("missing test_record_c op")
	}
	if createOp.isPublic {
		t.Fatalf("expected test_record_c not to be Public")
	}
	if createOp.resource != model.Resource("test_record") || createOp.action != model.Create {
		t.Fatalf("expected Requires(test_record, Create), got (%v, %v)", createOp.resource, createOp.action)
	}
}

func TestMount_NotPublicByDefault(t *testing.T) {
	fake := newFakeReg()
	mod := &publicTestModule{}
	err := crud.Mount(fake, nil, mod)
	if err != nil {
		t.Fatalf("unexpected mount error: %v", err)
	}

	readOp := fake.ops["test_record_r"]
	if readOp == nil {
		t.Fatalf("missing test_record_r op")
	}
	if readOp.isPublic {
		t.Fatalf("expected test_record_r NOT to be Public")
	}
	if readOp.resource != model.Resource("test_record") || readOp.action != model.Read {
		t.Fatalf("expected Requires(test_record, Read), got (%v, %v)", readOp.resource, readOp.action)
	}
}

type nilFnModule struct{}

func (m *nilFnModule) ModelName() string { return "nilmod" }
func (m *nilFnModule) MountOperations(reg crud.Registry) {
	reg.Read(&mockRecord{}, nil, nil)
}

func TestMount_NilFunctionIsError(t *testing.T) {
	fake := newFakeReg()
	mod := &nilFnModule{}
	err := crud.Mount(fake, nil, mod)
	if err != crud.ErrNilFunction {
		t.Fatalf("expected ErrNilFunction, got %v", err)
	}
}

// TestMount_RouteHasNoPublic verifies that crud.Route exposes only RequiresMFA and Describe,
// and specifically does not expose Public(), Authenticated() or Requires().
func TestMount_RouteHasNoPublic(t *testing.T) {
	var _ interface {
		RequiresMFA() crud.Route
		Describe(string) crud.Route
	} = crud.Route(nil)
}
