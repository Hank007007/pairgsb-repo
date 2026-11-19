// Package lease tracks secret hand-outs so they can be revoked per backend.
package lease

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrNotFound is returned for unknown lease identifiers.
var ErrNotFound = errors.New("lease not found")

// ErrRevoked is returned when a lease has been revoked.
var ErrRevoked = errors.New("lease revoked")

// Lease records one successful secret hand-out.
type Lease struct {
	ID        string
	Accessor  string
	Path      string
	Backend   string
	IssuedAt  time.Time
	Revoked   bool
	RevokedAt time.Time
}

func cacheKey(backend, path string) string {
	return backend + "\x00" + path
}

// Manager owns all leases, the per-backend value cache, and revocation state.
// It is safe for concurrent use.
type Manager struct {
	mu      sync.Mutex
	leases  map[string]*Lease
	byBack  map[string][]string
	cache   map[string]string
	nextID  int
	nowFunc func() time.Time
}

// NewManager returns an empty lease manager.
func NewManager() *Manager {
	return &Manager{
		leases:  make(map[string]*Lease),
		byBack:  make(map[string][]string),
		cache:   make(map[string]string),
		nowFunc: time.Now,
	}
}

// Issue creates a lease for accessor/path/backend and returns it.
func (m *Manager) Issue(accessor, path, backend string) *Lease {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	l := &Lease{
		ID:       fmt.Sprintf("lease_%06d", m.nextID),
		Accessor: accessor,
		Path:     path,
		Backend:  backend,
		IssuedAt: m.nowFunc(),
	}
	m.leases[l.ID] = l
	m.byBack[backend] = append(m.byBack[backend], l.ID)
	copied := *l
	return &copied
}

// Get returns a lease by id.
func (m *Manager) Get(id string) (*Lease, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.leases[id]
	if !ok {
		return nil, false
	}
	copied := *l
	return &copied, true
}

// Check verifies that a lease may serve a read for accessor at path.
func (m *Manager) Check(id, accessor, path string) (*Lease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.leases[id]
	if !ok {
		return nil, ErrNotFound
	}
	if l.Revoked {
		return nil, ErrRevoked
	}
	if l.Accessor != accessor || l.Path != path {
		return nil, ErrNotFound
	}
	copied := *l
	return &copied, nil
}

// CacheValue remembers the secret value served by a backend at path.
func (m *Manager) CacheValue(backend, path, value string) {
	m.mu.Lock()
	m.cache[cacheKey(backend, path)] = value
	m.mu.Unlock()
}

// CachedValue returns the value cached for a backend at path.
func (m *Manager) CachedValue(backend, path string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.cache[cacheKey(backend, path)]
	return v, ok
}

// LeasesFor lists the lease ids bound to a backend, oldest first.
func (m *Manager) LeasesFor(backend string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := m.byBack[backend]
	out := make([]string, len(ids))
	copy(out, ids)
	return out
}

// Revoke revokes a backend instance and returns the number of leases newly
// marked revoked.
func (m *Manager) Revoke(backend string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	revoked := 0
	for _, id := range m.byBack[backend] {
		l, ok := m.leases[id]
		if !ok || l.Revoked {
			continue
		}
		l.Revoked = true
		l.RevokedAt = m.nowFunc()
		revoked++
	}

	prefix := backend + "\x00"
	for key := range m.cache {
		if strings.HasPrefix(key, prefix) {
			delete(m.cache, key)
		}
	}
	return revoked
}
