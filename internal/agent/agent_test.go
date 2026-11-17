package agent_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sealrelay/internal/agent"
	"sealrelay/internal/control"
	"sealrelay/internal/nodekeys"
)

func TestAcceptMaterializesSecrets(t *testing.T) {
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
		Revision:     1,
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		RecipientID:  node.ID,
		RecipientPub: node.Public,
		Secrets: []control.Secret{
			{Name: "api-token", Value: []byte("tok-123")},
			{Name: "db-password", Value: []byte("hunter2")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	receiver, err := agent.NewReceiver(root, node.ID, node.Private, signPub)
	if err != nil {
		t.Fatal(err)
	}
	state, err := receiver.Accept(env)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if state.Bundle != "payments" || state.Revision != 1 {
		t.Fatalf("state = %+v", state)
	}
	if len(state.Names) != 2 {
		t.Fatalf("names = %v", state.Names)
	}

	for name, want := range map[string]string{
		"api-token":   "tok-123",
		"db-password": "hunter2",
	} {
		got, err := os.ReadFile(filepath.Join(root, agent.SecretsDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(got) != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestReceiverLoadsEmptyState(t *testing.T) {
	root := t.TempDir()
	state, err := agent.LoadState(root)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.Bundle != "" || state.Revision != 0 || len(state.Names) != 0 {
		t.Fatalf("state = %+v, want zero value", state)
	}
}
