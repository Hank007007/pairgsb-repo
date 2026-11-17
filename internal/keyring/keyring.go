// Package keyring manages the versioned key-encryption keys (KEKs) that
// protect object data keys.
package keyring

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// KeySize is the size in bytes of an AES-256 key.
const KeySize = 32

// Status describes how a key version may be used.
type Status string

const (
	// StatusActive keys may wrap new data keys.
	StatusActive Status = "active"
	// StatusRetired keys may only unwrap data keys that already exist.
	StatusRetired Status = "retired"
)

var (
	ErrEmpty            = errors.New("keyring contains no keys")
	ErrNoWriter         = errors.New("keyring has no active key for writing")
	ErrUnknownVersion   = errors.New("keyring has no such key version")
	ErrInvalidStatus    = errors.New("keyring has an invalid key status")
	ErrDuplicateVersion = errors.New("keyring has a duplicate key version")
	ErrInvalidVersion   = errors.New("keyring key version must be positive")
	ErrInvalidKeySize   = errors.New("keyring key must be exactly 32 bytes")
)

// Key is a single versioned KEK. Material is the raw key-encryption key.
type Key struct {
	Version  uint32
	Status   Status
	Material []byte
}

// Writable reports whether the key may be used to wrap new data keys.
func (k Key) Writable() bool {
	return k.Status == StatusActive
}

// Keyring is an ordered collection of key versions. It is safe for concurrent
// use.
type Keyring struct {
	mu   sync.RWMutex
	keys []Key
}

// New returns a keyring containing the supplied keys, ordered by version.
// The key material is copied so later mutation of the argument does not affect
// the keyring.
func New(keys []Key) *Keyring {
	r := &Keyring{keys: make([]Key, 0, len(keys))}
	for _, k := range keys {
		r.keys = append(r.keys, copyKey(k))
	}
	r.sort()
	return r
}

// Len reports the number of key versions in the keyring.
func (r *Keyring) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.keys)
}

// Keys returns a copy of the key versions, ordered by version.
func (r *Keyring) Keys() []Key {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Key, len(r.keys))
	for i, k := range r.keys {
		out[i] = copyKey(k)
	}
	return out
}

// NextVersion returns the next unused version number.
func (r *Keyring) NextVersion() uint32 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.nextVersionLocked()
}

// Add appends a key version to the keyring.
func (r *Keyring) Add(k Key) error {
	if err := validateKey(k); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.keys {
		if existing.Version == k.Version {
			return fmt.Errorf("%w: %d", ErrDuplicateVersion, k.Version)
		}
	}
	r.keys = append(r.keys, copyKey(k))
	r.sort()
	return nil
}

// Rotate adds k as the new active key and retires every key that was active
// before the call. Retired versions remain available for reading.
func (r *Keyring) Rotate(k Key) error {
	if err := validateKey(k); err != nil {
		return err
	}
	if k.Status != StatusActive {
		return fmt.Errorf("%w: rotation key must be active", ErrInvalidStatus)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.keys {
		if existing.Version == k.Version {
			return fmt.Errorf("%w: %d", ErrDuplicateVersion, k.Version)
		}
	}
	for i := range r.keys {
		if r.keys[i].Status == StatusActive {
			r.keys[i].Status = StatusRetired
		}
	}
	r.keys = append(r.keys, copyKey(k))
	r.sort()
	return nil
}

// String renders a keyring summary without exposing key material.
func (r *Keyring) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return fmt.Sprintf("keyring{versions:%d}", len(r.keys))
}

func (r *Keyring) nextVersionLocked() uint32 {
	var max uint32
	for _, k := range r.keys {
		if k.Version > max {
			max = k.Version
		}
	}
	return max + 1
}

func (r *Keyring) sort() {
	sort.Slice(r.keys, func(i, j int) bool { return r.keys[i].Version < r.keys[j].Version })
}

func validateKey(k Key) error {
	if k.Version == 0 {
		return ErrInvalidVersion
	}
	switch k.Status {
	case StatusActive, StatusRetired:
	default:
		return fmt.Errorf("%w: %q", ErrInvalidStatus, k.Status)
	}
	if len(k.Material) != KeySize {
		return fmt.Errorf("%w: got %d bytes", ErrInvalidKeySize, len(k.Material))
	}
	return nil
}

func copyKey(k Key) Key {
	return Key{
		Version:  k.Version,
		Status:   k.Status,
		Material: append([]byte(nil), k.Material...),
	}
}
