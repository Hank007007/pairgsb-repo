package control_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"sealrelay/internal/control"
	"sealrelay/internal/manifest"
	"sealrelay/internal/nodekeys"
)

func TestSealBuildsVerifiableEnvelope(t *testing.T) {
	signPub, signPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	node, err := nodekeys.Generate()
	if err != nil {
		t.Fatal(err)
	}
	sealer, err := control.NewSealer(signPriv)
	if err != nil {
		t.Fatal(err)
	}

	env, err := sealer.Seal(control.Request{
		Bundle:       "payments",
		Revision:     3,
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		RecipientID:  node.ID,
		RecipientPub: node.Public,
		Secrets: []control.Secret{
			{Name: "api-token", Value: []byte("tok-123")},
			{Name: "db-password", Value: []byte("hunter2")},
		},
	})
	if err != nil {
		t.Fatalf("seal: %v", err)
	}

	if env.Version != 1 {
		t.Fatalf("version = %d, want 1", env.Version)
	}
	if len(env.EphemeralPublic) != 32 {
		t.Fatalf("ephemeral public key length = %d, want 32", len(env.EphemeralPublic))
	}
	if len(env.Nonce) != 12 {
		t.Fatalf("nonce length = %d, want 12", len(env.Nonce))
	}
	if len(env.Ciphertext) == 0 {
		t.Fatal("ciphertext is empty")
	}
	if env.Manifest.Payload.Bundle != "payments" {
		t.Fatalf("bundle = %q, want payments", env.Manifest.Payload.Bundle)
	}
	if env.Manifest.Payload.Revision != 3 {
		t.Fatalf("revision = %d, want 3", env.Manifest.Payload.Revision)
	}
	if env.Manifest.Recipient != node.ID {
		t.Fatalf("recipient = %q, want %q", env.Manifest.Recipient, node.ID)
	}
	wantNames := []string{"api-token", "db-password"}
	if len(env.Manifest.Payload.Names) != len(wantNames) {
		t.Fatalf("names = %v, want %v", env.Manifest.Payload.Names, wantNames)
	}
	for i, name := range wantNames {
		if env.Manifest.Payload.Names[i] != name {
			t.Fatalf("names = %v, want %v", env.Manifest.Payload.Names, wantNames)
		}
	}
	if err := manifest.VerifyManifest(&env.Manifest, signPub); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestSealRejectsIncompleteRequest(t *testing.T) {
	_, signPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sealer, err := control.NewSealer(signPriv)
	if err != nil {
		t.Fatal(err)
	}
	node, err := nodekeys.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sealer.Seal(control.Request{
		Bundle:       "payments",
		RecipientID:  node.ID,
		RecipientPub: node.Public,
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	}); err == nil {
		t.Fatal("expected an error for a bundle with no secrets")
	}
}
