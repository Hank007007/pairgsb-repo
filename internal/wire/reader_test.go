package wire

import (
	"bytes"
	"testing"
)

func TestReaderReadByte(t *testing.T) {
	r := NewReader([]byte{0x01, 0x02})

	b, err := r.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	if b != 0x01 {
		t.Fatalf("byte = %#x, want 0x01", b)
	}
	if r.Remaining() != 1 {
		t.Fatalf("Remaining = %d, want 1", r.Remaining())
	}

	if _, err := r.ReadByte(); err != nil {
		t.Fatalf("second ReadByte: %v", err)
	}
	if _, err := r.ReadByte(); err != ErrUnexpectedEOF {
		t.Fatalf("err = %v, want ErrUnexpectedEOF", err)
	}
}

func TestReaderUVarintSingleByte(t *testing.T) {
	for _, tc := range []struct {
		in   byte
		want uint32
	}{
		{0x00, 0},
		{0x01, 1},
		{0x7f, 127},
	} {
		r := NewReader([]byte{tc.in})
		got, err := r.ReadUVarint()
		if err != nil {
			t.Fatalf("%#x: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("%#x: got %d, want %d", tc.in, got, tc.want)
		}
		if r.Remaining() != 0 {
			t.Fatalf("%#x: Remaining = %d, want 0", tc.in, r.Remaining())
		}
	}
}

func TestReaderUVarintMultiByte(t *testing.T) {
	for _, tc := range []struct {
		in   []byte
		want uint32
	}{
		{[]byte{0x80, 0x01}, 128},
		{[]byte{0xff, 0x01}, 255},
		{[]byte{0xe5, 0x8e, 0x26}, 624485},
		{[]byte{0x80, 0x80, 0x04}, 65536},
	} {
		r := NewReader(tc.in)
		got, err := r.ReadUVarint()
		if err != nil {
			t.Fatalf("%x: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("%x: got %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestReaderUVarintMaxU32(t *testing.T) {
	r := NewReader([]byte{0xff, 0xff, 0xff, 0xff, 0x0f})
	got, err := r.ReadUVarint()
	if err != nil {
		t.Fatalf("ReadUVarint: %v", err)
	}
	if got != 0xffffffff {
		t.Fatalf("got %d, want %d", got, uint32(0xffffffff))
	}
}

func TestReaderUVarintTruncated(t *testing.T) {
	r := NewReader([]byte{0x80})
	if _, err := r.ReadUVarint(); err != ErrUnexpectedEOF {
		t.Fatalf("err = %v, want ErrUnexpectedEOF", err)
	}
}

func TestReaderReadBytes(t *testing.T) {
	r := NewReader([]byte{0x03, 'a', 'b', 'c'})
	got, err := r.ReadBytes()
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if !bytes.Equal(got, []byte("abc")) {
		t.Fatalf("got %q, want %q", got, "abc")
	}
	if r.Remaining() != 0 {
		t.Fatalf("Remaining = %d, want 0", r.Remaining())
	}
}

func TestReaderReadBytesEmpty(t *testing.T) {
	r := NewReader([]byte{0x00, 0xff})
	got, err := r.ReadBytes()
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
	if r.Remaining() != 1 {
		t.Fatalf("Remaining = %d, want 1", r.Remaining())
	}
}

func TestReaderReadString(t *testing.T) {
	r := NewReader([]byte{0x04, 'n', 'o', 'd', 'e'})
	got, err := r.ReadString()
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	if got != "node" {
		t.Fatalf("got %q, want %q", got, "node")
	}
}
