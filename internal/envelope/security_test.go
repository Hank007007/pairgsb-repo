package envelope

import (
	"bytes"
	"testing"

	"kekring/internal/keyring"
)

func TestOpenRejectsDifferentObjectID(t *testing.T) {
	kr := newTestKeyring(t)
	blob, err := Seal(kr, "bucket/a", []byte("belongs to a"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	if _, err := Open(kr, "bucket/b", blob); err == nil {
		t.Fatal("Open accepted an envelope for a different object id")
	}
}

func TestOpenRejectsTransplantedHeader(t *testing.T) {
	kr := newTestKeyring(t)

	aBlob, err := Seal(kr, "bucket/a", []byte("plaintext a"))
	if err != nil {
		t.Fatalf("Seal a: %v", err)
	}
	bBlob, err := Seal(kr, "bucket/b", []byte("plaintext b"))
	if err != nil {
		t.Fatalf("Seal b: %v", err)
	}

	aEnv, err := Parse(aBlob)
	if err != nil {
		t.Fatalf("Parse a: %v", err)
	}
	bEnv, err := Parse(bBlob)
	if err != nil {
		t.Fatalf("Parse b: %v", err)
	}

	transplanted := &Envelope{
		Header:    aEnv.Header,
		BodyNonce: bEnv.BodyNonce,
		Body:      bEnv.Body,
	}
	blob, err := transplanted.Marshal()
	if err != nil {
		t.Fatalf("Marshal transplanted envelope: %v", err)
	}

	if _, err := Open(kr, "bucket/b", blob); err == nil {
		t.Fatal("Open accepted object A's header transplanted onto object B")
	}
}

func TestOpenRejectsChangedKEKVersion(t *testing.T) {
	kr := newTestKeyring(t)
	blob, err := Seal(kr, "bucket/obj", []byte("payload"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	env, err := Parse(blob)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	env.Header.KEKVersion = 2
	blob, err = env.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	sameMaterial := bytes.Repeat([]byte{0x5a}, 32)
	krV2 := keyringWithKey(t, 2, sameMaterial)
	if _, err := Open(krV2, "bucket/obj", blob); err == nil {
		t.Fatal("Open accepted a header whose KEK version was changed")
	}
}

func TestSealChangesDEKNonceForSameObject(t *testing.T) {
	kr := newTestKeyring(t)

	first, err := Seal(kr, "bucket/obj", []byte("payload"))
	if err != nil {
		t.Fatalf("first Seal: %v", err)
	}
	second, err := Seal(kr, "bucket/obj", []byte("payload"))
	if err != nil {
		t.Fatalf("second Seal: %v", err)
	}

	firstEnv, err := Parse(first)
	if err != nil {
		t.Fatalf("Parse first: %v", err)
	}
	secondEnv, err := Parse(second)
	if err != nil {
		t.Fatalf("Parse second: %v", err)
	}

	if bytes.Equal(firstEnv.Header.DEKNonce, secondEnv.Header.DEKNonce) {
		t.Fatal("sealing the same object twice reused the data key nonce")
	}
}

func TestRewrapUsesWriteKeyAndFreshNonce(t *testing.T) {
	kr := newTestKeyring(t)
	blob, err := Seal(kr, "bucket/obj", []byte("body must not move"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	before, err := Parse(blob)
	if err != nil {
		t.Fatalf("Parse before: %v", err)
	}

	if err := kr.Rotate(keyring.Key{
		Version:  2,
		Status:   keyring.StatusActive,
		Material: bytes.Repeat([]byte{0x6b}, keyring.KeySize),
	}); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	rewrapped, err := Rewrap(kr, "bucket/obj", blob)
	if err != nil {
		t.Fatalf("Rewrap: %v", err)
	}
	after, err := Parse(rewrapped)
	if err != nil {
		t.Fatalf("Parse after: %v", err)
	}

	if after.Header.KEKVersion != 2 {
		t.Errorf("KEKVersion after rewrap = %d, want 2", after.Header.KEKVersion)
	}
	if bytes.Equal(before.Header.DEKNonce, after.Header.DEKNonce) {
		t.Error("Rewrap reused the previous data key nonce")
	}
	if !bytes.Equal(before.BodyNonce, after.BodyNonce) {
		t.Error("Rewrap changed the body nonce")
	}
	if !bytes.Equal(before.Body, after.Body) {
		t.Error("Rewrap changed the body ciphertext")
	}

	got, err := Open(kr, "bucket/obj", rewrapped)
	if err != nil {
		t.Fatalf("Open after rewrap: %v", err)
	}
	if !bytes.Equal(got, []byte("body must not move")) {
		t.Fatal("plaintext differs after rewrap")
	}
}

func keyringWithKey(t *testing.T, version uint32, material []byte) *keyring.Keyring {
	t.Helper()
	return keyring.New([]keyring.Key{{
		Version:  version,
		Status:   keyring.StatusActive,
		Material: material,
	}})
}
