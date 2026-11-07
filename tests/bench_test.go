package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/json"
	"webtyp.com/model"
)

var sinkOp string

func BenchmarkOp(b *testing.B) {
	rec := &mockRecord{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkOp = crud.Op(rec, model.Read)
	}
}

func BenchmarkCap(b *testing.B) {
	b.ReportAllocs()
	var sink int64
	for i := 0; i < b.N; i++ {
		sink = crud.Cap(20)
		sink += crud.Cap(0)
		sink += crud.Cap(9999)
	}
	_ = sink
}

func BenchmarkReadArgs_Encode(b *testing.B) {
	args := crud.ReadArgs{Search: "router-core", After: "01J9Z8X7W6V5U4T3S2R1Q0P9N8", Limit: 50}
	var out string
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := json.Encode(&args, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadArgs_EncodeBytes(b *testing.B) {
	args := crud.ReadArgs{Search: "router-core", After: "01J9Z8X7W6V5U4T3S2R1Q0P9N8", Limit: 50}
	var out []byte
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := json.Encode(&args, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadArgs_Decode(b *testing.B) {
	raw := []byte(`{"search":"router-core","after":"01J9Z8X7W6V5U4T3S2R1Q0P9N8","limit":50}`)
	var args crud.ReadArgs
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := json.Decode(raw, &args); err != nil {
			b.Fatal(err)
		}
	}
}

var sinkPointers []any

func BenchmarkReadArgs_Pointers(b *testing.B) {
	var f model.Fielder = &crud.ReadArgs{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkPointers = f.Pointers()
	}
}

func BenchmarkCreateArgs_Encode_1(b *testing.B) {
	rec := &mockRecord{ID: "01J9Z8X7W6V5", Name: "Core Switch 01"}
	args := crud.CreateArgs{Records: []model.Model{rec}}
	var out []byte
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := json.Encode(&args, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCreateArgs_Encode_10(b *testing.B) {
	recs := make([]model.Model, 10)
	for i := range recs {
		recs[i] = &mockRecord{ID: "01J9Z8X7W6V5", Name: "Core Switch"}
	}
	args := crud.CreateArgs{Records: recs}
	var out []byte
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := json.Encode(&args, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCreateArgs_Encode_100(b *testing.B) {
	recs := make([]model.Model, 100)
	for i := range recs {
		recs[i] = &mockRecord{ID: "01J9Z8X7W6V5", Name: "Core Switch"}
	}
	args := crud.CreateArgs{Records: recs}
	var out []byte
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := json.Encode(&args, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUpdateArgs_Encode(b *testing.B) {
	rec := &mockRecord{ID: "01J9Z8X7W6V5", Name: "Patched Name"}
	args := crud.UpdateArgs{
		IDs:    []string{"01J9Z8X7W6V5", "01J9Z8X7W6V6", "01J9Z8X7W6V7"},
		Fields: []string{"name"},
		Record: rec,
	}
	var out []byte
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := json.Encode(&args, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDeleteArgs_Encode_1(b *testing.B) {
	args := crud.DeleteArgs{IDs: []string{"01J9Z8X7W6V5"}}
	var out []byte
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := json.Encode(&args, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDeleteArgs_Encode_10(b *testing.B) {
	ids := make([]string, 10)
	for i := range ids {
		ids[i] = "01J9Z8X7W6V5"
	}
	args := crud.DeleteArgs{IDs: ids}
	var out []byte
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := json.Encode(&args, &out); err != nil {
			b.Fatal(err)
		}
	}
}

type benchModule struct{}

func (m *benchModule) ModelName() string { return "bench" }
func (m *benchModule) MountOperations(reg crud.Registry) {
	reg.Create(func() model.ModelSlice { return &mockRecordList{} }, func(c crud.Caller, list model.ModelSlice) error { return nil })
	reg.Read(&mockRecord{}, nil, func(c crud.Caller, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		return &mockRecordList{{ID: "01J9Z8X7W6V5", Name: "Core Switch"}}, nil
	})
	reg.Update(func() model.Model { return &mockRecord{} }, func(c crud.Caller, ids, fields []string, r model.Model) error { return nil })
	reg.Delete(&mockRecord{}, func(c crud.Caller, ids []string) error { return nil })
}

func BenchmarkCreate_Loopback_1(b *testing.B) {
	mod := &benchModule{}
	c := crud.LoopbackActingAs("u", mod)
	op := "bench." + crud.Op(&mockRecord{}, model.Create)
	args := crud.CreateArgs{Records: []model.Model{&mockRecord{ID: "01J9Z8X7W6V5", Name: "Core Switch"}}}
	var out mockRecord
	nop := func(error) {}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Call(op, &args, &out, nop)
	}
}

func BenchmarkCreate_Loopback_10(b *testing.B) {
	mod := &benchModule{}
	c := crud.LoopbackActingAs("u", mod)
	op := "bench." + crud.Op(&mockRecord{}, model.Create)
	recs := make([]model.Model, 10)
	for i := range recs {
		recs[i] = &mockRecord{ID: "01J9Z8X7W6V5", Name: "Core Switch"}
	}
	args := crud.CreateArgs{Records: recs}
	var out mockRecord
	nop := func(error) {}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Call(op, &args, &out, nop)
	}
}

func BenchmarkCreate_Loopback_100(b *testing.B) {
	mod := &benchModule{}
	c := crud.LoopbackActingAs("u", mod)
	op := "bench." + crud.Op(&mockRecord{}, model.Create)
	recs := make([]model.Model, 100)
	for i := range recs {
		recs[i] = &mockRecord{ID: "01J9Z8X7W6V5", Name: "Core Switch"}
	}
	args := crud.CreateArgs{Records: recs}
	var out mockRecord
	nop := func(error) {}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Call(op, &args, &out, nop)
	}
}

func BenchmarkRead_Loopback_1(b *testing.B) {
	mod := &benchModule{}
	c := crud.LoopbackActingAs("u", mod)
	op := "bench." + crud.Op(&mockRecord{}, model.Read)
	args := crud.ReadArgs{Search: "Core", Limit: 20}
	var out mockRecordList
	nop := func(error) {}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out = out[:0]
		c.Call(op, &args, &out, nop)
	}
}

func BenchmarkUpdate_Loopback_1(b *testing.B) {
	mod := &benchModule{}
	c := crud.LoopbackActingAs("u", mod)
	op := "bench." + crud.Op(&mockRecord{}, model.Update)
	args := crud.UpdateArgs{
		IDs:    []string{"01J9Z8X7W6V5"},
		Fields: []string{"name"},
		Record: &mockRecord{ID: "01J9Z8X7W6V5", Name: "Patched"},
	}
	nop := func(error) {}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Call(op, &args, nil, nop)
	}
}

func BenchmarkDelete_Loopback_1(b *testing.B) {
	mod := &benchModule{}
	c := crud.LoopbackActingAs("u", mod)
	op := "bench." + crud.Op(&mockRecord{}, model.Delete)
	args := crud.DeleteArgs{IDs: []string{"01J9Z8X7W6V5"}}
	nop := func(error) {}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Call(op, &args, nil, nop)
	}
}

func BenchmarkDelete_Loopback_10(b *testing.B) {
	mod := &benchModule{}
	c := crud.LoopbackActingAs("u", mod)
	op := "bench." + crud.Op(&mockRecord{}, model.Delete)
	ids := make([]string, 10)
	for i := range ids {
		ids[i] = "01J9Z8X7W6V5"
	}
	args := crud.DeleteArgs{IDs: ids}
	nop := func(error) {}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Call(op, &args, nil, nop)
	}
}
