package envelope

import (
	"bytes"
	"testing"

	"kekring/internal/keyring"
)

func newTestKeyring(t *testing.T) *keyring.Keyring {
	t.Helper()
	return keyring.New([]keyring.Key{{
		Version:  1,
		Status:   keyring.StatusActive,
		Material: bytes.Repeat([]byte{0x5a}, keyring.KeySize),
	}})
}

func TestSealOpenRoundTrip(t *testing.T) {
	kr := newTestKeyring(t)
	plaintext := []byte("the quick brown fox jumps over the lazy dog")

	blob, err := Seal(kr, "bucket/obj", plaintext)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	got, err := Open(kr, "bucket/obj", blob)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatal("plaintext differs after seal and open")
	}
}

func TestSealUsesWriteKeyVersion(t *testing.T) {
	kr := newTestKeyring(t)

	blob, err := Seal(kr, "bucket/obj", []byte("payload"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	env, err := Parse(blob)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if env.Header.KEKVersion != 1 {
		t.Fatalf("KEKVersion = %d, want 1", env.Header.KEKVersion)
	}
	if env.Header.Algorithm != "AES-256-GCM" {
		t.Errorf("Algorithm = %q, want AES-256-GCM", env.Header.Algorithm)
	}
	if len(env.Header.DEKNonce) != NonceSize {
		t.Errorf("DEKNonce is %d bytes, want %d", len(env.Header.DEKNonce), NonceSize)
	}
}

func TestOpenAfterRotationStillWorks(t *testing.T) {
	kr := newTestKeyring(t)

	oldBlob, err := Seal(kr, "bucket/obj", []byte("written before rotation"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	err = kr.Rotate(keyring.Key{
		Version:  2,
		Status:   keyring.StatusActive,
		Material: bytes.Repeat([]byte{0x6b}, keyring.KeySize),
	})
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	got, err := Open(kr, "bucket/obj", oldBlob)
	if err != nil {
		t.Fatalf("Open after rotation: %v", err)
	}
	if !bytes.Equal(got, []byte("written before rotation")) {
		t.Fatal("plaintext differs after rotation")
	}

	newBlob, err := Seal(kr, "bucket/obj", []byte("written after rotation"))
	if err != nil {
		t.Fatalf("Seal after rotation: %v", err)
	}
	gotNew, err := Open(kr, "bucket/obj", newBlob)
	if err != nil {
		t.Fatalf("Open new object after rotation: %v", err)
	}
	if !bytes.Equal(gotNew, []byte("written after rotation")) {
		t.Fatal("plaintext of new object differs after rotation")
	}
}

func TestRewrapKeepsBodyBytes(t *testing.T) {
	kr := newTestKeyring(t)

	before, err := Seal(kr, "bucket/obj", []byte("body must not move"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	beforeEnv, err := Parse(before)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if err := kr.Rotate(keyring.Key{
		Version:  2,
		Status:   keyring.StatusActive,
		Material: bytes.Repeat([]byte{0x7c}, keyring.KeySize),
	}); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	after, err := Rewrap(kr, "bucket/obj", before)
	if err != nil {
		t.Fatalf("Rewrap: %v", err)
	}
	afterEnv, err := Parse(after)
	if err != nil {
		t.Fatalf("Parse after rewrap: %v", err)
	}

	if !bytes.Equal(afterEnv.BodyNonce, beforeEnv.BodyNonce) {
		t.Error("body nonce changed during rewrap")
	}
	if !bytes.Equal(afterEnv.Body, beforeEnv.Body) {
		t.Error("body ciphertext changed during rewrap")
	}

	got, err := Open(kr, "bucket/obj", after)
	if err != nil {
		t.Fatalf("Open after rewrap: %v", err)
	}
	if !bytes.Equal(got, []byte("body must not move")) {
		t.Fatal("plaintext differs after rewrap")
	}
}

func TestOpenRejectsTruncatedBlob(t *testing.T) {
	kr := newTestKeyring(t)

	blob, err := Seal(kr, "bucket/obj", []byte("payload"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	if _, err := Parse(blob[:10]); err == nil {
		t.Fatal("Parse accepted a truncated blob")
	}
	if _, err := Open(kr, "bucket/obj", blob[:10]); err == nil {
		t.Fatal("Open accepted a truncated blob")
	}
}

func TestMarshalRejectsBadHeader(t *testing.T) {
	env := &Envelope{
		Header: Header{
			KEKVersion: 1,
			Algorithm:  "AES-256-GCM",
			DEKNonce:   make([]byte, 8),
			SealedDEK:  make([]byte, 48),
		},
		BodyNonce: make([]byte, NonceSize),
		Body:      make([]byte, TagSize),
	}
	if _, err := env.Marshal(); err == nil {
		t.Fatal("Marshal accepted an 8-byte data key nonce")
	}
}
