package cbor

import (
	"bytes"
	"testing"
)

func TestDecodeScalars(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want any
	}{
		{"uint zero", []byte{0x00}, uint64(0)},
		{"uint small", []byte{0x17}, uint64(23)},
		{"uint one byte", []byte{0x18, 0x64}, uint64(100)},
		{"uint two bytes", []byte{0x19, 0x01, 0x00}, uint64(256)},
		{"negative", []byte{0x20}, int64(-1)},
		{"false", []byte{0xf4}, false},
		{"true", []byte{0xf5}, true},
		{"null", []byte{0xf6}, nil},
		{"text", []byte{0x63, 'a', 'b', 'c'}, "abc"},
		{"bytes", []byte{0x43, 0x01, 0x02, 0x03}, []byte{1, 2, 3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Decode(tc.in)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			switch want := tc.want.(type) {
			case []byte:
				if !bytes.Equal(got.([]byte), want) {
					t.Fatalf("got %x, want %x", got, want)
				}
			default:
				if got != want {
					t.Fatalf("got %#v, want %#v", got, want)
				}
			}
		})
	}
}

func TestDecodeMapAndArray(t *testing.T) {
	in := []byte{0xa2, 0x61, 'a', 0x01, 0x61, 'b', 0x82, 0x02, 0x03}
	v, err := Decode(in)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	m, ok := v.(*Map)
	if !ok {
		t.Fatalf("got %T, want *Map", v)
	}
	if got, _ := m.Get("a"); got != uint64(1) {
		t.Fatalf("m[a] = %#v", got)
	}
	arr, ok := mustGet(t, m, "b").([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("m[b] = %#v", m)
	}
	if arr[0] != uint64(2) || arr[1] != uint64(3) {
		t.Fatalf("array = %#v", arr)
	}
	if m.Len() != 2 {
		t.Fatalf("Len = %d", m.Len())
	}
}

func TestDecodeRejectsTruncated(t *testing.T) {
	if _, err := Decode([]byte{0x18}); err == nil {
		t.Fatal("expected error for truncated integer")
	}
	if _, err := Decode([]byte{0x44, 0x01, 0x02}); err == nil {
		t.Fatal("expected error for truncated byte string")
	}
	if _, err := Decode([]byte{0x01, 0x02}); err == nil {
		t.Fatal("expected error for trailing bytes")
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	m := NewMap()
	m.Set("alpha", uint64(1))
	m.Set("beta", []byte{0xde, 0xad})
	m.Set("gamma", []any{"x", true, nil})

	raw, err := Encode(m)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	back, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	bm, ok := back.(*Map)
	if !ok {
		t.Fatalf("got %T, want *Map", back)
	}
	if got, _ := bm.Get("alpha"); got != uint64(1) {
		t.Fatalf("alpha = %#v", got)
	}
	if got, _ := bm.Get("beta"); !bytes.Equal(got.([]byte), []byte{0xde, 0xad}) {
		t.Fatalf("beta = %#v", got)
	}
}

func TestEncodeUsesShortestForm(t *testing.T) {
	raw, err := Encode(uint64(5))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(raw, []byte{0x05}) {
		t.Fatalf("got %x, want 05", raw)
	}
}

func mustGet(t *testing.T, m *Map, key string) any {
	t.Helper()
	v, ok := m.Get(key)
	if !ok {
		t.Fatalf("missing key %q", key)
	}
	return v
}
