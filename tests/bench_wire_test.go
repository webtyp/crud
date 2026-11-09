package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/json"
	"webtyp.com/model"
)

// Wire benchmarks: what the client (WASM) and the server pay to encode and decode the arguments of each operation. Run: go test ./tests -run '^$' -bench Wire -benchmem
// Names carry the Wire prefix so one -bench pattern selects them; results are recorded in docs/PERFORMANCE.md.

var sinkPointers []any

func BenchmarkWire_Cap(b *testing.B) {
	b.ReportAllocs()
	var sink int64
	for i := 0; i < b.N; i++ {
		sink = crud.Cap(20)
		sink += crud.Cap(0)
		sink += crud.Cap(9999)
	}
	_ = sink
}

func BenchmarkWire_ReadArgs_Encode(b *testing.B) {
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

func BenchmarkWire_ReadArgs_EncodeBytes(b *testing.B) {
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

func BenchmarkWire_ReadArgs_Decode(b *testing.B) {
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

func BenchmarkWire_ReadArgs_Pointers(b *testing.B) {
	var f model.Fielder = &crud.ReadArgs{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkPointers = f.Pointers()
	}
}

func BenchmarkWire_CreateArgs_Encode_1(b *testing.B) {
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

func BenchmarkWire_CreateArgs_Encode_10(b *testing.B) {
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

func BenchmarkWire_CreateArgs_Encode_100(b *testing.B) {
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

func BenchmarkWire_UpdateArgs_Encode(b *testing.B) {
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

func BenchmarkWire_DeleteArgs_Encode_1(b *testing.B) {
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

func BenchmarkWire_DeleteArgs_Encode_10(b *testing.B) {
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
