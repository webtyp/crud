package tests

import (
	"strconv"
	"testing"

	"webtyp.com/crud"
	"webtyp.com/model"
)

// The benchmarks measure what one request costs through crud (decode, validate, rules, transaction, store, encode) over the in-memory store,
// so the numbers are crud's own cost: a real database adds its own time on top. Run: go test ./tests -run "^$" -bench Engine -benchmem

func benchModule(withRule bool) (*testModule, *eventRecorder) {
	rec := &eventRecorder{}
	entity := crud.StoredEntity{
		Model:   &tRec{},
		NewList: func() model.ModelSlice { return &tRecList{} },
		Store:   newMemStore(),
		Rules:   declareRules(model.AllActions, 0),
	}
	if withRule {
		entity.Rules = declareRules(model.AllActions, model.Create|model.Update|model.Delete)
		entity.Create = func(a crud.CurrentUser, tx crud.Tx, r model.Model) (crud.RuleAnswer, error) {
			return crud.ApplyAndPublish(), nil
		}
		entity.Update = func(a crud.CurrentUser, tx crud.Tx, old, next model.Model, fields []string) (crud.RuleAnswer, error) {
			return crud.ApplyAndPublish(), nil
		}
		entity.Delete = func(a crud.CurrentUser, tx crud.Tx, r model.Model) (crud.RuleAnswer, error) {
			return crud.ApplyAndPublish(), nil
		}
	}
	return &testModule{table: entity}, rec
}

// Creating into a store that keeps growing measures the store's growth, not crud (the in-memory test store copies itself for its rollback),
// so a create is measured in steady state: each create is deleted before the next, and the store stays empty.
func benchCreateDelete(b *testing.B, withRule bool) {
	mod, rec := benchModule(withRule)
	caller := crud.LoopbackActingAs(rec, "u1", mod)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var saved tRec
		caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{Name: "n"}}}, &saved, func(error) {})
		caller.Call(operationOf(model.Delete), &crud.DeleteArgs{IDs: []string{saved.ID}}, nil, func(error) {})
		rec.got = rec.got[:0]
	}
}

func BenchmarkEngine_CreateAndDelete_NoRule(b *testing.B)   { benchCreateDelete(b, false) }
func BenchmarkEngine_CreateAndDelete_WithRule(b *testing.B) { benchCreateDelete(b, true) }

func BenchmarkEngine_ReadOneByID(b *testing.B) {
	mod, _ := benchModule(false)
	caller := crud.LoopbackActingAs(nil, "u1", mod)
	var saved tRec
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{Name: "n"}}}, &saved, func(error) {})
	id := saved.ID
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var got tRecList
		caller.Call(operationOf(model.Read), &crud.ReadArgs{ID: id}, &got, func(error) {})
	}
}

func BenchmarkEngine_ReadPageOf50(b *testing.B) {
	mod, _ := benchModule(false)
	caller := crud.LoopbackActingAs(nil, "u1", mod)
	for i := 0; i < 500; i++ {
		caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{Name: "n" + strconv.Itoa(i)}}}, nil, func(error) {})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var got tRecList
		caller.Call(operationOf(model.Read), &crud.ReadArgs{Limit: 50}, &got, func(error) {})
	}
}

func BenchmarkEngine_UpdateOneField(b *testing.B) {
	mod, rec := benchModule(false)
	caller := crud.LoopbackActingAs(rec, "u1", mod)
	var saved tRec
	caller.Call(operationOf(model.Create), &crud.CreateArgs{Records: []model.Model{&tRec{Name: "n"}}}, &saved, func(error) {})
	id := saved.ID
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		caller.Call(operationOf(model.Update), &crud.UpdateArgs{IDs: []string{id}, Fields: []string{"name"}, Record: &tRec{Name: "m"}}, nil, func(error) {})
		rec.got = rec.got[:0]
	}
}

// BenchmarkBaseline_StoreOnly is what a create plus a delete cost with no crud at all (the memory store directly): the difference is crud's overhead.
func BenchmarkEngine_Baseline_StoreOnly(b *testing.B) {
	store := newMemStore()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := &tRec{Name: "n"}
		r.ID = store.NewID()
		_ = store.Create(r)
		_ = store.Delete(r, r.ID)
	}
}
