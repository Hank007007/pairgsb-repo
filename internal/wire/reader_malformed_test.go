package wire

import (
	"errors"
	"testing"
)

func TestReaderUVarintOverflow(t *testing.T) {
	cases := map[string][]byte{
		"overflow bits in fifth byte": {0xff, 0xff, 0xff, 0xff, 0x1f},
		"high nibble set fifth byte":  {0xff, 0xff, 0xff, 0xff, 0x7f},
		"six byte encoding":           {0x80, 0x80, 0x80, 0x80, 0x80, 0x01},
		"max u64 style run":           {0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReader(in)
			if _, err := r.ReadUVarint(); !errors.Is(err, ErrVarintOverflow) {
				t.Fatalf("ReadUVarint(% x): err = %v, want ErrVarintOverflow", in, err)
			}
		})
	}
}

func TestReaderUVarintFifthByteBoundary(t *testing.T) {
	// The largest legal u32 encoding: the fifth byte uses only its low
	// four bits and has no continuation bit.
	r := NewReader([]byte{0xff, 0xff, 0xff, 0xff, 0x0f})
	got, err := r.ReadUVarint()
	if err != nil {
		t.Fatalf("ReadUVarint: %v", err)
	}
	if got != uint32(0xffffffff) {
		t.Fatalf("got %d, want %d", got, uint32(0xffffffff))
	}

	// A continuation bit on a legal fifth byte still demands a sixth byte.
	if _, err := NewReader([]byte{0xff, 0xff, 0xff, 0xff, 0x8f}).ReadUVarint(); !errors.Is(err, ErrVarintOverflow) {
		t.Fatalf("continuation in fifth byte: err = %v, want ErrVarintOverflow", err)
	}
}

func TestReaderReadBytesLengthExceedsBuffer(t *testing.T) {
	// Declared length 5 with only two payload bytes must return the sentinel
	// error instead of slicing past the buffer and panicking.
	r := NewReader([]byte{0x05, 'a', 'b'})
	b, err := r.ReadBytes()
	if !errors.Is(err, ErrLengthExceedsBuffer) {
		t.Fatalf("err = %v, want ErrLengthExceedsBuffer", err)
	}
	if b != nil {
		t.Fatalf("returned bytes = %v, want nil on error", b)
	}

	// Length equal to the remaining bytes is still legal.
	r = NewReader([]byte{0x02, 'a', 'b'})
	b, err = r.ReadBytes()
	if err != nil {
		t.Fatalf("exact-length ReadBytes: %v", err)
	}
	if string(b) != "ab" {
		t.Fatalf("got %q, want %q", b, "ab")
	}
}

func TestReaderReadBytesTruncatedLength(t *testing.T) {
	// A multi-byte length whose varint itself runs past the buffer.
	r := NewReader([]byte{0x80})
	if _, err := r.ReadBytes(); !errors.Is(err, ErrUnexpectedEOF) {
		t.Fatalf("err = %v, want ErrUnexpectedEOF", err)
	}
}
