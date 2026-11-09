package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/json"
)

// The server never returns more than MaxPage rows, whatever the browser asks.
func TestCap(t *testing.T) {
	for _, tt := range []struct{ in, want int64 }{{0, crud.MaxPage}, {-1, crud.MaxPage}, {20, 20}, {crud.MaxPage + 1, crud.MaxPage}} {
		if got := crud.Cap(tt.in); got != tt.want {
			t.Errorf("Cap(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

// ReadArgs travels the wire both ways unchanged (including optional ID).
func TestReadArgs_RoundTrip(t *testing.T) {
	in := crud.ReadArgs{ID: "rec-9", Search: "ana", After: "17", Limit: 20}
	var buf []byte
	if err := json.Encode(&in, &buf); err != nil {
		t.Fatalf("encode: %v", err)
	}
	var out crud.ReadArgs
	if err := json.Decode(buf, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out != in {
		t.Errorf("ReadArgs round trip: got %+v, want %+v", out, in)
	}
}

func TestReadArgs_EncodesOnlySetFields(t *testing.T) {
	wEmpty := &recordingFieldWriter{}
	empty := crud.ReadArgs{}
	empty.EncodeFields(wEmpty)
	if len(wEmpty.objects) != 0 || len(wEmpty.arrays) != 0 || len(wEmpty.strs) != 0 {
		t.Errorf("expected no fields encoded for empty ReadArgs, got objects=%v arrays=%v", wEmpty.objects, wEmpty.arrays)
	}

	wSearch := &recordingFieldWriter{}
	searchOnly := crud.ReadArgs{Search: "x"}
	searchOnly.EncodeFields(wSearch)
	// it writes only search as String("search", "x")
	if len(wSearch.objects) != 0 || len(wSearch.arrays) != 0 {
		t.Errorf("expected no objects/arrays for searchOnly, got objects=%v arrays=%v", wSearch.objects, wSearch.arrays)
	}
}
