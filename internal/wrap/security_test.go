package wrap

import (
	"bytes"
	"testing"
)

func TestUnwrapDEKRejectsDifferentObjectID(t *testing.T) {
	kek := testKEK(0x11)
	sealed, err := WrapDEK(kek, 1, "bucket/a", Algorithm, testDEK(0x22))
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}

	if _, err := UnwrapDEK(kek, 1, "bucket/b", Algorithm, sealed.Nonce, sealed.Ciphertext); err == nil {
		t.Fatal("UnwrapDEK accepted a sealed DEK for a different object id")
	}
}

func TestUnwrapDEKRejectsDifferentVersion(t *testing.T) {
	kek := testKEK(0x11)
	sealed, err := WrapDEK(kek, 1, "bucket/a", Algorithm, testDEK(0x22))
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}

	if _, err := UnwrapDEK(kek, 2, "bucket/a", Algorithm, sealed.Nonce, sealed.Ciphertext); err == nil {
		t.Fatal("UnwrapDEK accepted a sealed DEK for a different version")
	}
}

func TestUnwrapDEKRejectsDifferentAlgorithm(t *testing.T) {
	kek := testKEK(0x11)
	sealed, err := WrapDEK(kek, 1, "bucket/a", Algorithm, testDEK(0x22))
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}

	if _, err := UnwrapDEK(kek, 1, "bucket/a", "AES-128-GCM", sealed.Nonce, sealed.Ciphertext); err == nil {
		t.Fatal("UnwrapDEK accepted a sealed DEK for a different algorithm")
	}
}

func TestWrapDEKNonceIsRandom(t *testing.T) {
	kek := testKEK(0x11)
	dek := testDEK(0x22)

	first, err := WrapDEK(kek, 1, "bucket/a", Algorithm, dek)
	if err != nil {
		t.Fatalf("first WrapDEK: %v", err)
	}
	second, err := WrapDEK(kek, 1, "bucket/a", Algorithm, dek)
	if err != nil {
		t.Fatalf("second WrapDEK: %v", err)
	}

	if bytes.Equal(first.Nonce, second.Nonce) {
		t.Fatal("wrapping the same DEK twice reused a nonce")
	}
}

func TestRewrapDEKChangesNonce(t *testing.T) {
	kek := testKEK(0x11)
	dek := testDEK(0x22)

	sealed, err := WrapDEK(kek, 1, "bucket/a", Algorithm, dek)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}
	rewrapped, err := RewrapDEK(kek, 1, "bucket/a", Algorithm, sealed.Nonce, sealed.Ciphertext)
	if err != nil {
		t.Fatalf("RewrapDEK: %v", err)
	}

	if bytes.Equal(sealed.Nonce, rewrapped.Nonce) {
		t.Fatal("RewrapDEK reused the previous nonce")
	}

	got, err := UnwrapDEK(kek, 1, "bucket/a", Algorithm, rewrapped.Nonce, rewrapped.Ciphertext)
	if err != nil {
		t.Fatalf("UnwrapDEK after rewrap: %v", err)
	}
	if !bytes.Equal(got, dek) {
		t.Fatal("rewrapped data key no longer opens to the original")
	}
}
