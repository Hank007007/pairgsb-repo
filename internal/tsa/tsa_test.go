package tsa

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthoritySignsRequest(t *testing.T) {
	auth, err := NewAuthority()
	if err != nil {
		t.Fatalf("authority: %v", err)
	}
	srv := httptest.NewServer(auth.Handler())
	defer srv.Close()

	client := NewClient(srv.URL, nil)
	tok, err := client.Stamp(context.Background(), Request{FromSeq: 1, ToSeq: 3, RootHex: "deadbeef"})
	if err != nil {
		t.Fatalf("stamp: %v", err)
	}
	if tok.RootHex != "deadbeef" {
		t.Fatalf("token echoes root %q", tok.RootHex)
	}
	if err := Verify(tok, auth.PublicKey()); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if auth.Stamps() != 1 {
		t.Fatalf("authority issued %d tokens, want 1", auth.Stamps())
	}
}

func TestAuthorityOfflineFails(t *testing.T) {
	auth, err := NewAuthority()
	if err != nil {
		t.Fatalf("authority: %v", err)
	}
	srv := httptest.NewServer(auth.Handler())
	defer srv.Close()
	auth.SetOffline(true)

	client := NewClient(srv.URL, nil)
	if _, err := client.Stamp(context.Background(), Request{FromSeq: 1, ToSeq: 1, RootHex: "aa"}); err == nil {
		t.Fatal("stamp succeeded while offline")
	}
}

func TestVerifyRejectsTamperedToken(t *testing.T) {
	auth, err := NewAuthority()
	if err != nil {
		t.Fatalf("authority: %v", err)
	}
	tok := Token{RootHex: "aa", GenTime: time.Now().UTC().Format(time.RFC3339)}
	tok.Sig = base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
	if err := Verify(tok, auth.PublicKey()); err == nil {
		t.Fatal("verify accepted a zero signature")
	}
}

func TestLoadOrCreateAuthorityIsStable(t *testing.T) {
	path := t.TempDir() + "/tsa.key"
	a, err := LoadOrCreateAuthority(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	b, err := LoadOrCreateAuthority(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if a.PublicKeyHex() != b.PublicKeyHex() {
		t.Fatal("reload produced a different key pair")
	}
}
