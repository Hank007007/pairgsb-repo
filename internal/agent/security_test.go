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
	"sealrelay/internal/envelope"
	"sealrelay/internal/manifest"
	"sealrelay/internal/nodekeys"
)

type fixture struct {
	signPub ed25519.PublicKey
	sealer  *control.Sealer
	node    *nodekeys.Node
	root    string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
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
	return &fixture{
		signPub: signPub,
		sealer:  sealer,
		node:    node,
		root:    t.TempDir(),
	}
}

func (f *fixture) seal(t *testing.T, revision uint64, expiresAt int64, secrets map[string]string) *envelope.Envelope {
	t.Helper()
	list := make([]control.Secret, 0, len(secrets))
	for name, value := range secrets {
		list = append(list, control.Secret{Name: name, Value: []byte(value)})
	}
	env, err := f.sealer.Seal(control.Request{
		Bundle:       "payments",
		Revision:     revision,
		ExpiresAt:    expiresAt,
		RecipientID:  f.node.ID,
		RecipientPub: f.node.Public,
		Secrets:      list,
	})
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func (f *fixture) receiver(t *testing.T) *agent.Receiver {
	t.Helper()
	r, err := agent.NewReceiver(f.root, f.node.ID, f.node.Private, f.signPub)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestVerifyRejectsRecipientAndExpiryTampering(t *testing.T) {
	f := newFixture(t)
	env := f.seal(t, 1, time.Now().Add(time.Hour).Unix(), map[string]string{"api-token": "tok"})

	env.Manifest.Recipient = "n-somebody-else"
	if err := manifest.VerifyManifest(&env.Manifest, f.signPub); err == nil {
		t.Fatal("verification accepted a retargeted envelope")
	}

	env = f.seal(t, 1, time.Now().Add(time.Hour).Unix(), map[string]string{"api-token": "tok"})
	env.Manifest.ExpiresAt = 1000000000
	if err := manifest.VerifyManifest(&env.Manifest, f.signPub); err == nil {
		t.Fatal("verification accepted an edited expiry")
	}
}

func TestAcceptRejectsExpiredAndExtendedExpiry(t *testing.T) {
	f := newFixture(t)
	receiver := f.receiver(t)

	expired := f.seal(t, 1, time.Now().Add(-time.Minute).Unix(), map[string]string{"api-token": "tok"})
	if _, err := receiver.Accept(expired); err == nil {
		t.Fatal("accepted an expired envelope")
	}

	tampered := f.seal(t, 2, time.Now().Add(-time.Minute).Unix(), map[string]string{"api-token": "tok"})
	tampered.Manifest.ExpiresAt = time.Now().Add(time.Hour).Unix()
	if _, err := receiver.Accept(tampered); err == nil {
		t.Fatal("accepted an envelope with an unsigned extended expiry")
	}
	if _, err := os.Stat(filepath.Join(f.root, agent.SecretsDir, "api-token")); !os.IsNotExist(err) {
		t.Fatalf("secret was written after rejection: %v", err)
	}
}

func TestAcceptRejectsLowerRevisionAndPreservesState(t *testing.T) {
	f := newFixture(t)
	receiver := f.receiver(t)
	expires := time.Now().Add(time.Hour).Unix()

	if _, err := receiver.Accept(f.seal(t, 2, expires, map[string]string{"api-token": "tok-v2"})); err != nil {
		t.Fatalf("accept revision 2: %v", err)
	}
	if _, err := receiver.Accept(f.seal(t, 1, expires, map[string]string{"api-token": "tok-v1"})); err == nil {
		t.Fatal("accepted a lower revision")
	}

	got, err := os.ReadFile(filepath.Join(f.root, agent.SecretsDir, "api-token"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "tok-v2" {
		t.Fatalf("api-token = %q, want tok-v2", got)
	}
	state, err := agent.LoadState(f.root)
	if err != nil {
		t.Fatal(err)
	}
	if state.Revision != 2 {
		t.Fatalf("state revision = %d, want 2", state.Revision)
	}
}

func TestAcceptRemovesSecretsAbsentFromNewRevision(t *testing.T) {
	f := newFixture(t)
	receiver := f.receiver(t)
	expires := time.Now().Add(time.Hour).Unix()

	if _, err := receiver.Accept(f.seal(t, 1, expires, map[string]string{
		"api-token":   "tok-v1",
		"db-password": "hunter2",
	})); err != nil {
		t.Fatalf("accept revision 1: %v", err)
	}
	if _, err := receiver.Accept(f.seal(t, 2, expires, map[string]string{
		"api-token": "tok-v2",
	})); err != nil {
		t.Fatalf("accept revision 2: %v", err)
	}

	if _, err := os.Stat(filepath.Join(f.root, agent.SecretsDir, "db-password")); !os.IsNotExist(err) {
		t.Fatalf("stale secret still present: %v", err)
	}
	state, err := agent.LoadState(f.root)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Names) != 1 || state.Names[0] != "api-token" {
		t.Fatalf("state names = %v, want [api-token]", state.Names)
	}
}
