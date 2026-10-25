package utils

import "testing"

func TestSignedEndpointMinInt16(t *testing.T) {
	ok, err := NewSignedRanges[int16]("-32768")
	if err != nil {
		t.Fatal(err)
	}
	if len(ok) != 1 || ok[0].Start() != -32768 || ok[0].End() != -32768 {
		t.Fatalf("got %#v", ok)
	}
	if _, err := NewSignedRanges[int16]("-32769"); err == nil {
		t.Fatal("expected error for -32769")
	}
	if _, err := NewSignedRanges[int16]("32768"); err == nil {
		t.Fatal("expected error for 32768")
	}
	wide, err := NewSignedRanges[int16]("32767")
	if err != nil || wide[0].Start() != 32767 {
		t.Fatalf("32767: %v %#v", err, wide)
	}
}
