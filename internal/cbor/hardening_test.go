package cbor

import (
	"bytes"
	"testing"
)

func TestIsCanonicalChecksNestedMapOrder(t *testing.T) {
	// {"targets": [{"model": "P1", "size": 1}]}: model must follow size.
	in := []byte{
		0xa1, 0x67, 't', 'a', 'r', 'g', 'e', 't', 's',
		0x81, 0xa2,
		0x65, 'm', 'o', 'd', 'e', 'l', 0x62, 'P', '1',
		0x64, 's', 'i', 'z', 'e', 0x01,
	}
	if IsCanonical(in) {
		t.Fatal("model before size must not be canonical")
	}
}

func TestIsCanonicalRejectsNonShortestIntegers(t *testing.T) {
	cases := map[string][]byte{
		"five as eight bytes": {0x1b, 0, 0, 0, 0, 0, 0, 0, 5},
		"five as four bytes":  {0x1a, 0, 0, 0, 5},
		"minus one as bytes":  {0x38, 0x00},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if IsCanonical(in) {
				t.Fatalf("%x must not be canonical", in)
			}
		})
	}
}

func TestIsCanonicalAcceptsEightByteInteger(t *testing.T) {
	// 2^32+1 in its shortest eight-byte form.
	in := []byte{0x1b, 0, 0, 0, 1, 0, 0, 0, 1}
	if !IsCanonical(in) {
		t.Fatal("expected canonical eight-byte integer")
	}
}

func TestIsCanonicalRejectsDuplicateKeys(t *testing.T) {
	// {"a": 1, "a": 2}
	in := []byte{0xa2, 0x61, 'a', 0x01, 0x61, 'a', 0x02}
	if IsCanonical(in) {
		t.Fatal("duplicate keys must not be canonical")
	}
}

func TestIsCanonicalRejectsIndefiniteLengths(t *testing.T) {
	cases := map[string][]byte{
		"array": {0x9f, 0x01, 0xff},
		"bytes": {0x5f, 0x41, 0x01, 0xff},
		"text":  {0x7f, 0x61, 'a', 0xff},
		"map":   {0xbf, 0x61, 'a', 0x01, 0xff},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if IsCanonical(in) {
				t.Fatalf("%x must not be canonical", in)
			}
		})
	}
}

func TestIsCanonicalRejectsNonShortestLengths(t *testing.T) {
	cases := map[string][]byte{
		"bytes": {0x58, 0x01, 0xaa},
		"text":  {0x78, 0x01, 'a'},
		"array": {0x98, 0x01, 0x00},
		"map":   {0xb8, 0x01, 0x61, 'a', 0x00},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if IsCanonical(in) {
				t.Fatalf("%x must not be canonical", in)
			}
		})
	}
}

func TestIsCanonicalOrdersKeysByEncodedBytes(t *testing.T) {
	// "z" encodes as 0x61 0x7a and sorts before "aa" (0x62 0x61 0x61).
	sorted := []byte{0xa2, 0x61, 'z', 0x01, 0x62, 'a', 'a', 0x02}
	if !IsCanonical(sorted) {
		t.Fatal("expected encoded-byte key order to be canonical")
	}
	unsorted := []byte{0xa2, 0x62, 'a', 'a', 0x02, 0x61, 'z', 0x01}
	if IsCanonical(unsorted) {
		t.Fatal("text order must not be used instead of encoded-byte order")
	}
}

func TestDecodeRejectsIndefiniteLengths(t *testing.T) {
	cases := map[string][]byte{
		"array": {0x9f, 0x01, 0x02, 0xff},
		"bytes": {0x5f, 0x41, 0x01, 0x41, 0x02, 0xff},
		"text":  {0x7f, 0x61, 'a', 0x61, 'b', 0xff},
		"map":   {0xbf, 0x61, 'a', 0x01, 0xff},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode(in); err == nil {
				t.Fatalf("Decode(%x) accepted indefinite-length input", in)
			}
		})
	}
}

func TestDecodeDepthAppliesToMapValues(t *testing.T) {
	var deep []byte
	for i := 0; i < 40; i++ {
		deep = append(deep, 0xa1, 0x61, 'a')
	}
	deep = append(deep, 0x00)
	if _, err := Decode(deep); err == nil {
		t.Fatal("deeply nested map values were accepted")
	}
}

func TestDecodeDepthBoundary(t *testing.T) {
	maps := func(n int) []byte {
		var out []byte
		for i := 0; i < n; i++ {
			out = append(out, 0xa1, 0x61, 'a')
		}
		return append(out, 0x00)
	}
	if _, err := Decode(maps(MaxDepth - 1)); err != nil {
		t.Fatalf("depth %d rejected: %v", MaxDepth-1, err)
	}
	if _, err := Decode(maps(MaxDepth)); err == nil {
		t.Fatalf("depth %d accepted", MaxDepth)
	}
}

func TestDecodeDepthAppliesToArrayElements(t *testing.T) {
	in := append(bytes.Repeat([]byte{0x81}, 40), 0x00)
	if _, err := Decode(in); err == nil {
		t.Fatal("deeply nested arrays were accepted")
	}
}

func TestNegativeIntegerBeyondInt64RoundTrips(t *testing.T) {
	in := []byte{0x3b, 0x80, 0, 0, 0, 0, 0, 0, 0}
	v, err := Decode(in)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	out, err := Encode(v)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(out, in) {
		t.Fatalf("round trip = %x, want %x", out, in)
	}
}
