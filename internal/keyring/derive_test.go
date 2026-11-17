package keyring

import (
	"bytes"
	"testing"
)

func TestDeriveKEKIsDeterministic(t *testing.T) {
	master := bytes.Repeat([]byte{0x42}, 32)

	first, err := DeriveKEK(master, 1)
	if err != nil {
		t.Fatalf("DeriveKEK: %v", err)
	}
	second, err := DeriveKEK(master, 1)
	if err != nil {
		t.Fatalf("DeriveKEK: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Fatal("DeriveKEK is not deterministic for a fixed master and version")
	}
	if len(first) != KeySize {
		t.Fatalf("derived key is %d bytes, want %d", len(first), KeySize)
	}
}

func TestDeriveKEKRejectsShortMaster(t *testing.T) {
	if _, err := DeriveKEK(bytes.Repeat([]byte{0x42}, 8), 1); err == nil {
		t.Fatal("DeriveKEK accepted a short master secret")
	}
}

func TestDeriveKEKRejectsZeroVersion(t *testing.T) {
	if _, err := DeriveKEK(bytes.Repeat([]byte{0x42}, 32), 0); err == nil {
		t.Fatal("DeriveKEK accepted version zero")
	}
}
