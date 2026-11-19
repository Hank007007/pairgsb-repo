package keystore

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"signward/internal/audit"
	"signward/internal/policy"
)

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := New(dir, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s, dir
}

func TestGenerateResolveAndSign(t *testing.T) {
	s, _ := newTestStore(t)
	p := policy.Policy{
		Purposes:             []string{"sign"},
		MaxSignaturesPerHour: 10,
		MaxPayloadBytes:      64,
	}

	handle, err := s.Generate("release-2026", p)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if handle == "" {
		t.Fatal("Generate returned an empty handle")
	}

	key, err := s.Resolve(handle)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if key.Handle != handle || key.ID != "release-2026" {
		t.Fatalf("Resolve returned handle=%q id=%q", key.Handle, key.ID)
	}
	if len(key.PublicKey()) != ed25519.PublicKeySize {
		t.Fatalf("public key has %d bytes, want %d", len(key.PublicKey()), ed25519.PublicKeySize)
	}

	msg := []byte("artifact digest")
	sig := key.Sign(msg)
	if !ed25519.Verify(key.PublicKey(), msg, sig) {
		t.Fatal("signature does not verify under the key's public key")
	}
}

func TestResolveUnknownHandle(t *testing.T) {
	s, _ := newTestStore(t)

	if _, err := s.Resolve("kh_does_not_exist"); !errors.Is(err, ErrUnknownHandle) {
		t.Fatalf("Resolve error = %v, want ErrUnknownHandle", err)
	}
}

func TestKeysSurviveReopen(t *testing.T) {
	dir := t.TempDir()
	p := policy.Policy{Purposes: []string{"sign"}, MaxSignaturesPerHour: 5, MaxPayloadBytes: 32}

	first, err := New(dir, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	handle, err := first.Generate("build-signer", p)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	second, err := New(dir, nil)
	if err != nil {
		t.Fatalf("reopen New: %v", err)
	}
	key, err := second.Resolve(handle)
	if err != nil {
		t.Fatalf("Resolve after reopen: %v", err)
	}

	msg := []byte("payload")
	if !ed25519.Verify(key.PublicKey(), msg, key.Sign(msg)) {
		t.Fatal("signature from reloaded key does not verify")
	}
}

func TestKeyFileIsWrittenWithRestrictedMode(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("file modes are not meaningful on this platform")
	}

	s, dir := newTestStore(t)
	handle, err := s.Generate("build-signer", policy.Policy{Purposes: []string{"sign"}})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, handle+".key"))
	if err != nil {
		t.Fatalf("Stat key file: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("key file mode = %o, want 600", mode)
	}
}

func TestDestroyRemovesKeyFile(t *testing.T) {
	s, dir := newTestStore(t)
	handle, err := s.Generate("retiring", policy.Policy{Purposes: []string{"sign"}})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if err := s.Destroy(handle); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, handle+".key")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("key file still present after Destroy: %v", err)
	}

	listed := s.List()
	if len(listed) != 1 {
		t.Fatalf("List returned %d keys, want 1", len(listed))
	}
	if listed[0].State != StateDestroyed {
		t.Fatalf("listed state = %q, want %q", listed[0].State, StateDestroyed)
	}
}

func TestDestroyUnknownHandle(t *testing.T) {
	s, _ := newTestStore(t)
	if err := s.Destroy("kh_missing"); !errors.Is(err, ErrUnknownHandle) {
		t.Fatalf("Destroy error = %v, want ErrUnknownHandle", err)
	}
}

func TestGenerateRejectsEmptyID(t *testing.T) {
	s, _ := newTestStore(t)
	if _, err := s.Generate("  ", policy.Policy{}); !errors.Is(err, ErrEmptyID) {
		t.Fatalf("Generate error = %v, want ErrEmptyID", err)
	}
}

func TestCreateAndDestroyAreAudited(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.log")
	log, err := audit.Open(logPath)
	if err != nil {
		t.Fatalf("audit.Open: %v", err)
	}
	defer log.Close()

	s, err := New(t.TempDir(), log)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	handle, err := s.Generate("audited", policy.Policy{Purposes: []string{"sign"}})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if err := s.Destroy(handle); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	events := readEvents(t, logPath)
	for _, want := range []audit.Event{audit.EventKeyCreated, audit.EventKeyDestroyed} {
		if !contains(events, want) {
			t.Fatalf("audit trail %v does not contain %q", events, want)
		}
	}
}

func readEvents(t *testing.T, path string) []audit.Event {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}

	var events []audit.Event
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var entry audit.Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("decode audit line %q: %v", line, err)
		}
		events = append(events, entry.Event)
	}
	return events
}

func contains(events []audit.Event, want audit.Event) bool {
	for _, e := range events {
		if e == want {
			return true
		}
	}
	return false
}
