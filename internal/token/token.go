// Package token mints and resolves accessor and wrap tokens.
package token

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrNotFound is returned when a wrap token is unknown, expired, or otherwise
// unusable.
var ErrNotFound = errors.New("token not found")

// DefaultTTL is used when a wrap request does not ask for a positive TTL.
const DefaultTTL = 300 * time.Second

// Wrap is a minted wrap token.
type Wrap struct {
	Value     string
	Path      string
	Accessor  string
	ExpiresAt time.Time
}

// Manager issues and resolves wrap tokens. It is safe for concurrent use.
type Manager struct {
	mu      sync.Mutex
	wraps   map[string]*Wrap
	nowFunc func() time.Time
}

// NewManager returns an empty wrap-token manager.
func NewManager() *Manager {
	return &Manager{
		wraps:   make(map[string]*Wrap),
		nowFunc: time.Now,
	}
}

func randomToken(prefix string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(buf), nil
}

// Mint creates a wrap token for accessor at path, valid for ttl seconds.
func (m *Manager) Mint(accessor, path string, ttl int64) (*Wrap, error) {
	value, err := randomToken("wrap_")
	if err != nil {
		return nil, fmt.Errorf("generate wrap token: %w", err)
	}
	now := m.nowFunc()
	var expires time.Time
	if ttl <= 0 {
		expires = now.Add(DefaultTTL)
	} else {
		expires = now.Add(time.Duration(ttl))
	}
	w := &Wrap{
		Value:     value,
		Path:      path,
		Accessor:  accessor,
		ExpiresAt: expires,
	}
	m.mu.Lock()
	m.wraps[value] = w
	m.mu.Unlock()
	return w, nil
}

// Unwrap resolves a wrap token. On success the returned Wrap describes the
// accessor and path the token was minted for.
func (m *Manager) Unwrap(value string) (*Wrap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.wraps[value]
	if !ok {
		return nil, ErrNotFound
	}
	if m.nowFunc().After(w.ExpiresAt) {
		return nil, ErrNotFound
	}
	return w, nil
}

// Close drops all outstanding wrap tokens.
func (m *Manager) Close() {
	m.mu.Lock()
	m.wraps = make(map[string]*Wrap)
	m.mu.Unlock()
}
