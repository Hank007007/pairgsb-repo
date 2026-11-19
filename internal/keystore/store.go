// Package keystore holds Ed25519 key material for the signing agent and
// hands it out by opaque handle.
//
// Keys are generated in-process from crypto/rand. The private seed is kept in
// memory and, so that a restarted agent can keep using the same handles, also
// written to a mode 0600 file inside the store directory. Nothing outside the
// process is able to read key material back out; see docs/agent-policy.md §6.
package keystore

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"signward/internal/audit"
	"signward/internal/policy"
)

// Errors returned by Store operations.
var (
	ErrUnknownHandle = errors.New("keystore: unknown key handle")
	ErrEmptyID       = errors.New("keystore: key id must not be empty")
	ErrKeyDestroyed  = errors.New("keystore: key has been destroyed")
	ErrKeyExpired    = errors.New("keystore: key has expired")
)

// State is the lifecycle state of a loaded key.
type State string

const (
	StateActive    State = "active"
	StateDestroyed State = "destroyed"
)

// Key is a usable key together with the policy that governs it.
type Key struct {
	Handle    string
	ID        string
	Policy    policy.Policy
	Public    ed25519.PublicKey
	CreatedAt time.Time

	private ed25519.PrivateKey
}

// Sign returns the Ed25519 signature of msg under k.
func (k *Key) Sign(msg []byte) []byte {
	return ed25519.Sign(k.private, msg)
}

// PublicKey returns a copy of the verification key.
func (k *Key) PublicKey() ed25519.PublicKey {
	out := make(ed25519.PublicKey, len(k.Public))
	copy(out, k.Public)
	return out
}

// Seed returns the private seed backing k.
func (k *Key) Seed() []byte {
	return k.private.Seed()
}

// Info is a non-secret description of a key held by the store.
type Info struct {
	Handle    string        `json:"handle"`
	ID        string        `json:"id"`
	State     State         `json:"state"`
	CreatedAt time.Time     `json:"createdAt"`
	Policy    policy.Policy `json:"policy"`
}

// Store is a set of keys addressed by handle. A Store is safe for concurrent
// use.
type Store struct {
	mu      sync.Mutex
	dir     string
	entries map[string]*entry
	audit   *audit.Log
	now     func() time.Time
}

type entry struct {
	handle      string
	id          string
	policy      policy.Policy
	private     ed25519.PrivateKey
	public      ed25519.PublicKey
	state       State
	createdAt   time.Time
	destroyedAt time.Time
}

// keyFile is the on-disk representation. The seed is base64-encoded and the
// file is created with mode 0600.
type keyFile struct {
	Version   int           `json:"version"`
	ID        string        `json:"id"`
	Handle    string        `json:"handle"`
	CreatedAt time.Time     `json:"createdAt"`
	Policy    policy.Policy `json:"policy"`
	Seed      string        `json:"seed"`
}

const keyFileVersion = 1

// New opens (creating it if needed) the store directory dir and loads every
// key file found inside it. audit may be nil, in which case no audit records
// are written.
func New(dir string, log *audit.Log) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("keystore: create directory: %w", err)
	}
	s := &Store{
		dir:     dir,
		entries: make(map[string]*entry),
		audit:   log,
		now:     time.Now,
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	paths, err := filepath.Glob(filepath.Join(s.dir, "*.key"))
	if err != nil {
		return fmt.Errorf("keystore: scan directory: %w", err)
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("keystore: read %s: %w", path, err)
		}
		var kf keyFile
		if err := json.Unmarshal(raw, &kf); err != nil {
			return fmt.Errorf("keystore: parse %s: %w", path, err)
		}
		if kf.Handle == "" || kf.ID == "" {
			return fmt.Errorf("keystore: %s: missing handle or id", path)
		}
		if _, dup := s.entries[kf.Handle]; dup {
			return fmt.Errorf("keystore: duplicate handle %q", kf.Handle)
		}
		seed, err := base64.StdEncoding.DecodeString(kf.Seed)
		if err != nil {
			return fmt.Errorf("keystore: %s: decode seed: %w", path, err)
		}
		priv := ed25519.NewKeyFromSeed(seed)
		e := &entry{
			handle:    kf.Handle,
			id:        kf.ID,
			policy:    kf.Policy,
			private:   priv,
			public:    priv.Public().(ed25519.PublicKey),
			state:     StateActive,
			createdAt: kf.CreatedAt,
		}
		s.entries[e.handle] = e
		s.audit.Record(audit.EventKeyLoaded, e.handle, "", e.id)
	}
	return nil
}

// Generate creates a new key, persists it and returns its handle.
func (s *Store) Generate(id string, p policy.Policy) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", ErrEmptyID
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("keystore: generate key: %w", err)
	}
	handle, err := newHandle()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	kf := keyFile{
		Version:   keyFileVersion,
		ID:        id,
		Handle:    handle,
		CreatedAt: now,
		Policy:    p,
		Seed:      base64.StdEncoding.EncodeToString(priv.Seed()),
	}
	raw, err := json.MarshalIndent(kf, "", "  ")
	if err != nil {
		return "", fmt.Errorf("keystore: encode key file: %w", err)
	}
	if err := os.WriteFile(s.path(handle), raw, 0o600); err != nil {
		return "", fmt.Errorf("keystore: write key file: %w", err)
	}

	s.entries[handle] = &entry{
		handle:    handle,
		id:        id,
		policy:    p,
		private:   priv,
		public:    pub,
		state:     StateActive,
		createdAt: now,
	}
	s.audit.RecordAt(now, audit.EventKeyCreated, handle, "", id)
	return handle, nil
}

// Resolve returns the key identified by handle using the store's clock.
func (s *Store) Resolve(handle string) (*Key, error) {
	return s.ResolveAt(handle, s.now())
}

// ResolveAt returns the key identified by handle if it is still usable at
// now. Unknown, destroyed and expired keys are refused and audited.
func (s *Store) ResolveAt(handle string, now time.Time) (*Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[handle]
	if !ok {
		s.audit.RecordAt(now, audit.EventResolveDenied, handle, "", "unknown key handle")
		return nil, ErrUnknownHandle
	}
	if e.state == StateDestroyed {
		s.audit.RecordAt(now, audit.EventResolveDenied, handle, "", "key has been destroyed")
		return nil, ErrKeyDestroyed
	}
	if e.policy.Expired(now) {
		s.audit.RecordAt(now, audit.EventResolveDenied, handle, "", "key has expired")
		return nil, ErrKeyExpired
	}
	return &Key{
		Handle:    e.handle,
		ID:        e.id,
		Policy:    e.policy,
		Public:    e.public,
		CreatedAt: e.createdAt,
		private:   e.private,
	}, nil
}

// PublicKey returns a copy of the verification key for handle. It does not
// apply the lifecycle checks used by Resolve because public keys are not
// secret and creation needs to report one for a key that may already be past
// its NotAfter instant.
func (s *Store) PublicKey(handle string) (ed25519.PublicKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[handle]
	if !ok {
		return nil, ErrUnknownHandle
	}
	out := make(ed25519.PublicKey, len(e.public))
	copy(out, e.public)
	return out, nil
}

// RefuseExport records an audit entry for every key and never returns key
// material. It is used when a caller attempts to request an export.
func (s *Store) RefuseExport() {
	s.mu.Lock()
	handles := make([]string, 0, len(s.entries))
	for _, e := range s.entries {
		handles = append(handles, e.handle)
	}
	s.mu.Unlock()

	if len(handles) == 0 {
		s.audit.Record(audit.EventExportDenied, "", "", "key export is not permitted")
		return
	}
	for _, handle := range handles {
		s.audit.Record(audit.EventExportDenied, handle, "", "key export is not permitted")
	}
}

// Destroy retires the key identified by handle and removes its seed file.
// Destroying an already-destroyed key is not an error.
func (s *Store) Destroy(handle string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[handle]
	if !ok {
		return ErrUnknownHandle
	}
	if e.state == StateDestroyed {
		return nil
	}
	e.state = StateDestroyed
	e.destroyedAt = s.now()
	if err := os.Remove(s.path(handle)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("keystore: remove key file: %w", err)
	}
	s.audit.RecordAt(e.destroyedAt, audit.EventKeyDestroyed, e.handle, "", e.id)
	return nil
}

// List returns a description of every key in the store, in no particular
// order.
func (s *Store) List() []Info {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Info, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, Info{
			Handle:    e.handle,
			ID:        e.id,
			State:     e.state,
			CreatedAt: e.createdAt,
			Policy:    e.policy,
		})
	}
	return out
}

func (s *Store) path(handle string) string {
	return filepath.Join(s.dir, handle+".key")
}

func newHandle() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("keystore: generate handle: %w", err)
	}
	return "kh_" + hex.EncodeToString(buf), nil
}
