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
