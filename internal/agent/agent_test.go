package agent

import (
	"crypto/ed25519"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"signward/internal/audit"
	"signward/internal/keystore"
	"signward/internal/policy"
)

func newTestAgent(t *testing.T) (*Agent, *keystore.Store, string) {
	t.Helper()

	log, err := audit.Open(filepath.Join(t.TempDir(), "audit.log"))
	if err != nil {
		t.Fatalf("audit.Open: %v", err)
	}
	t.Cleanup(func() { log.Close() })

	store, err := keystore.New(t.TempDir(), log)
	if err != nil {
		t.Fatalf("keystore.New: %v", err)
	}

	handle, err := store.Generate("build-signer", policy.Policy{
		Purposes:             []string{"sign"},
		MaxSignaturesPerHour: 100,
		MaxPayloadBytes:      1024,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return New(store, log), store, handle
}

func TestSignAfterUnlock(t *testing.T) {
	a, store, handle := newTestAgent(t)

	if err := a.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	payload := []byte("sha256:c0ffee")
	resp, err := a.Sign(Request{KeyHandle: handle, Purpose: "sign", Payload: payload})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if resp.KeyHandle != handle || resp.Purpose != "sign" {
		t.Fatalf("response = %+v", resp)
	}

	key, err := store.Resolve(handle)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !ed25519.Verify(key.PublicKey(), payload, resp.Signature) {
		t.Fatal("returned signature does not verify under the key")
	}
}

func TestSignWithoutUnlockIsRefused(t *testing.T) {
	a, _, handle := newTestAgent(t)

	if !a.Locked() {
		t.Fatal("a freshly constructed agent reports an open session")
	}
	_, err := a.Sign(Request{KeyHandle: handle, Purpose: "sign", Payload: []byte("x")})
	if !errors.Is(err, ErrSessionLocked) {
		t.Fatalf("Sign error = %v, want ErrSessionLocked", err)
	}
}

func TestLockClosesSession(t *testing.T) {
	a, _, handle := newTestAgent(t)

	if err := a.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if a.Locked() {
		t.Fatal("Locked reports true right after Unlock")
	}
	if err := a.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if !a.Locked() {
		t.Fatal("Locked reports false after Lock")
	}

	_, err := a.Sign(Request{KeyHandle: handle, Purpose: "sign", Payload: []byte("x")})
	if !errors.Is(err, ErrSessionLocked) {
		t.Fatalf("Sign error = %v, want ErrSessionLocked", err)
	}
}

func TestSignUnderQuotaLimit(t *testing.T) {
	log, err := audit.Open(filepath.Join(t.TempDir(), "audit.log"))
	if err != nil {
		t.Fatalf("audit.Open: %v", err)
	}
	defer log.Close()

	store, err := keystore.New(t.TempDir(), log)
	if err != nil {
		t.Fatalf("keystore.New: %v", err)
	}
	handle, err := store.Generate("limited", policy.Policy{
		Purposes:             []string{"sign"},
		MaxSignaturesPerHour: 3,
		MaxPayloadBytes:      64,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	a := New(store, log)
	if err := a.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	for i := 0; i < 3; i++ {
		if _, err := a.Sign(Request{KeyHandle: handle, Purpose: "sign", Payload: []byte("x")}); err != nil {
			t.Fatalf("Sign #%d under the cap: %v", i+1, err)
		}
	}
}

func TestSignRejectsOversizedPayload(t *testing.T) {
	log, err := audit.Open(filepath.Join(t.TempDir(), "audit.log"))
	if err != nil {
		t.Fatalf("audit.Open: %v", err)
	}
	defer log.Close()

	store, err := keystore.New(t.TempDir(), log)
	if err != nil {
		t.Fatalf("keystore.New: %v", err)
	}
	handle, err := store.Generate("small", policy.Policy{
		Purposes:             []string{"sign"},
		MaxSignaturesPerHour: 10,
		MaxPayloadBytes:      4,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	a := New(store, log)
	if err := a.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	_, err = a.Sign(Request{KeyHandle: handle, Purpose: "sign", Payload: []byte("too long")})
	if !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("Sign error = %v, want ErrPayloadTooLarge", err)
	}
}

func TestSignRejectsUnknownHandle(t *testing.T) {
	a, _, _ := newTestAgent(t)

	if err := a.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	_, err := a.Sign(Request{KeyHandle: "kh_nope", Purpose: "sign", Payload: []byte("x")})
	if !errors.Is(err, keystore.ErrUnknownHandle) {
		t.Fatalf("Sign error = %v, want ErrUnknownHandle", err)
	}
}

func TestSessionTTLIsFifteenMinutes(t *testing.T) {
	if SessionTTL != 15*time.Minute {
		t.Fatalf("SessionTTL = %v, want 15m", SessionTTL)
	}
}
