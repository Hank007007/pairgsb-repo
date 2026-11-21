// Package keystore stores the salts used to build and open commitments.
package keystore

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// SaltID is the identifier written into a record next to a commitment.
type SaltID string

var saltIDPattern = regexp.MustCompile(`^[0-9a-f]{16,64}$`)

// ErrNotFound is returned when a salt id is not present in the store.
var ErrNotFound = errors.New("keystore: salt not found")

// Entry is everything kept for one redacted field: the salt that produced the
// commitment and the sealed copy of the original value.
type Entry struct {
	Salt       []byte
	Ciphertext []byte
}

// Store is a directory-backed salt store. Each entry is one file named after
// its id. The file holds two hex lines: the salt and the sealed value.
type Store struct {
	dir string
	mu  sync.Mutex
}

// Open prepares dir for use as a salt store and returns it.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("keystore: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Dir reports the directory backing the store.
func (s *Store) Dir() string {
	return s.dir
}

// NewSaltID returns a random identifier in the format accepted by the store.
func NewSaltID() (SaltID, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("keystore: %w", err)
	}
	return SaltID(hex.EncodeToString(buf)), nil
}

// Put writes entry under id. An existing id is not overwritten.
func (s *Store) Put(id SaltID, entry Entry) error {
	if !saltIDPattern.MatchString(string(id)) {
		return fmt.Errorf("keystore: invalid salt id %q", id)
	}
	if len(entry.Salt) == 0 {
		return errors.New("keystore: empty salt")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.dir, string(id)+".salt")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("keystore: %w", err)
	}
	body := hex.EncodeToString(entry.Salt) + "\n" + hex.EncodeToString(entry.Ciphertext) + "\n"
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		return fmt.Errorf("keystore: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("keystore: %w", err)
	}
	return f.Close()
}

// Get returns the entry stored under id.
func (s *Store) Get(id SaltID) (Entry, error) {
	if !saltIDPattern.MatchString(string(id)) {
		return Entry{}, fmt.Errorf("keystore: invalid salt id %q", id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(filepath.Join(s.dir, string(id)+".salt"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Entry{}, fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return Entry{}, fmt.Errorf("keystore: %w", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\r\n"), "\n")
	if len(lines) < 1 || lines[0] == "" {
		return Entry{}, fmt.Errorf("keystore: corrupt salt %s", id)
	}
	salt, err := hex.DecodeString(strings.TrimSpace(lines[0]))
	if err != nil {
		return Entry{}, fmt.Errorf("keystore: corrupt salt %s: %w", id, err)
	}
	entry := Entry{Salt: salt}
	if len(lines) > 1 && strings.TrimSpace(lines[1]) != "" {
		ct, err := hex.DecodeString(strings.TrimSpace(lines[1]))
		if err != nil {
			return Entry{}, fmt.Errorf("keystore: corrupt sealed value %s: %w", id, err)
		}
		entry.Ciphertext = ct
	}
	return entry, nil
}

// Has reports whether id is present.
func (s *Store) Has(id SaltID) bool {
	_, err := s.Get(id)
	return err == nil
}
