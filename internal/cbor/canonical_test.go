package cbor

import "testing"

func TestIsCanonicalAcceptsSortedTopLevel(t *testing.T) {
	// {"a": 1, "b": 2}
	in := []byte{0xa2, 0x61, 'a', 0x01, 0x61, 'b', 0x02}
	if !IsCanonical(in) {
		t.Fatal("expected canonical")
	}
}

func TestIsCanonicalAcceptsNestedSortedTargets(t *testing.T) {
	// {"targets": [{"size": 1, "model": "P1"}]} with keys in encoded order:
	// "size" encodes as 0x64..., "model" as 0x65....
	in := []byte{
		0xa1, 0x67, 't', 'a', 'r', 'g', 'e', 't', 's',
		0x81, 0xa2,
		0x64, 's', 'i', 'z', 'e', 0x01,
		0x65, 'm', 'o', 'd', 'e', 'l', 0x62, 'P', '1',
	}
	if !IsCanonical(in) {
		t.Fatal("expected canonical")
	}
}

func TestIsCanonicalAcceptsNestedMapInArray(t *testing.T) {
	// {"a": [{"k": 1}]}
	in := []byte{0xa1, 0x61, 'a', 0x81, 0xa1, 0x61, 'k', 0x01}
	if !IsCanonical(in) {
		t.Fatal("expected canonical")
	}
}

func TestIsCanonicalAcceptsLongInteger(t *testing.T) {
	// 0x1a 00 01 00 00 is 65536 in its shortest four-byte form.
	in := []byte{0x1a, 0x00, 0x01, 0x00, 0x00}
	if !IsCanonical(in) {
		t.Fatal("expected canonical")
	}
}

func TestIsCanonicalAcceptsEncoderOutput(t *testing.T) {
	m := NewMap()
	m.Set("zeta", uint64(300))
	m.Set("alpha", []any{uint64(1), uint64(2)})
	raw, err := Encode(m)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !IsCanonical(raw) {
		t.Fatalf("encoder output rejected: %x", raw)
	}
}
