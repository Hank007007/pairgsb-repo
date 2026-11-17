package wrap

import (
	"bytes"
	"testing"

	"kekring/internal/keyring"
)

func testKEK(fill byte) []byte {
	return bytes.Repeat([]byte{fill}, keyring.KeySize)
}

func testDEK(fill byte) []byte {
	return bytes.Repeat([]byte{fill}, DEKSize)
}

func TestWrapUnwrapRoundTrip(t *testing.T) {
	kek := testKEK(0x11)
	dek := testDEK(0x22)

	sealed, err := WrapDEK(kek, 1, "bucket/obj", Algorithm, dek)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}
	if sealed.Algorithm != Algorithm {
		t.Errorf("Algorithm = %q, want %q", sealed.Algorithm, Algorithm)
	}
	if sealed.Version != 1 {
		t.Errorf("Version = %d, want 1", sealed.Version)
	}
	if len(sealed.Nonce) != nonceSize {
		t.Errorf("Nonce is %d bytes, want %d", len(sealed.Nonce), nonceSize)
	}
	if len(sealed.Ciphertext) != DEKSize+16 {
		t.Errorf("Ciphertext is %d bytes, want %d", len(sealed.Ciphertext), DEKSize+16)
	}

	got, err := UnwrapDEK(kek, 1, "bucket/obj", Algorithm, sealed.Nonce, sealed.Ciphertext)
	if err != nil {
		t.Fatalf("UnwrapDEK: %v", err)
	}
	if !bytes.Equal(got, dek) {
		t.Fatal("unwrapped data key differs from the original")
	}
}

func TestUnwrapRejectsWrongKEK(t *testing.T) {
	sealed, err := WrapDEK(testKEK(0x11), 1, "bucket/obj", Algorithm, testDEK(0x22))
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}

	if _, err := UnwrapDEK(testKEK(0x33), 1, "bucket/obj", Algorithm, sealed.Nonce, sealed.Ciphertext); err == nil {
		t.Fatal("UnwrapDEK accepted the wrong key")
	}
}

func TestWrapRejectsWrongDEKSize(t *testing.T) {
	if _, err := WrapDEK(testKEK(0x11), 1, "bucket/obj", Algorithm, make([]byte, 16)); err == nil {
		t.Fatal("WrapDEK accepted a 16-byte data key")
	}
}

func TestWrapRejectsUnknownAlgorithm(t *testing.T) {
	if _, err := WrapDEK(testKEK(0x11), 1, "bucket/obj", "ChaCha20-Poly1305", testDEK(0x22)); err == nil {
		t.Fatal("WrapDEK accepted an unsupported algorithm")
	}
}

func TestRewrapDEKRoundTrip(t *testing.T) {
	kek := testKEK(0x11)
	dek := testDEK(0x22)

	sealed, err := WrapDEK(kek, 1, "bucket/obj", Algorithm, dek)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}

	rewrapped, err := RewrapDEK(kek, 1, "bucket/obj", Algorithm, sealed.Nonce, sealed.Ciphertext)
	if err != nil {
		t.Fatalf("RewrapDEK: %v", err)
	}

	got, err := UnwrapDEK(kek, 1, "bucket/obj", Algorithm, rewrapped.Nonce, rewrapped.Ciphertext)
	if err != nil {
		t.Fatalf("UnwrapDEK after rewrap: %v", err)
	}
	if !bytes.Equal(got, dek) {
		t.Fatal("rewrapped data key no longer opens to the original")
	}
}
