package middleware

import (
	"sync"
	"time"

	"scopewall/internal/policy"
)

// Source supplies the policy document in force. It reloads the document from disk when the
// cached copy is older than the refresh interval.
type Source struct {
	mu        sync.Mutex
	loader    func() (*policy.Policy, error)
	refresh   time.Duration
	current   *policy.Policy
	loadErr   error
	loadedAt  time.Time
	hasLoaded bool
	now       func() time.Time
}

// NewFileSource builds a source reading the policy document at path. A non-positive refresh
// interval means the document is read once and kept.
func NewFileSource(path string, refresh time.Duration) *Source {
	return &Source{
		loader:  func() (*policy.Policy, error) { return policy.LoadFile(path) },
		refresh: refresh,
		now:     time.Now,
	}
}

// NewSource builds a source over a custom loader, useful for tests and embedded policies.
func NewSource(loader func() (*policy.Policy, error), refresh time.Duration) *Source {
	return &Source{loader: loader, refresh: refresh, now: time.Now}
}

// Policy returns the document in force, reloading it when due.
func (s *Source) Policy() (*policy.Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.hasLoaded && (s.refresh <= 0 || s.now().Sub(s.loadedAt) < s.refresh) {
		return s.current, s.loadErr
	}

	p, err := s.loader()
	s.current = p
	s.loadErr = err
	s.loadedAt = s.now()
	s.hasLoaded = true
	return p, err
}
