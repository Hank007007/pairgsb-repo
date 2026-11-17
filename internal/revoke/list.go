// Package revoke tracks token identifiers that must no longer be honoured.
package revoke

import (
	"sync"

	"capbaton/internal/token"
)

// List is a concurrency-safe set of revoked token identifiers.
type List struct {
	mu  sync.RWMutex
	ids map[string]struct{}
}

// New returns an empty revocation list.
func New() *List {
	return &List{ids: make(map[string]struct{})}
}

// Revoke adds tokenID to the list. Revoking an identifier twice is a
// no-op.
func (l *List) Revoke(tokenID string) {
	if tokenID == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.ids == nil {
		l.ids = make(map[string]struct{})
	}
	l.ids[tokenID] = struct{}{}
}

// Has reports whether tokenID is on the list.
func (l *List) Has(tokenID string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, ok := l.ids[tokenID]
	return ok
}

// Len returns the number of revoked identifiers.
func (l *List) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.ids)
}

// IsRevoked reports whether the chain has been revoked.
func (l *List) IsRevoked(links []token.Link) bool {
	if l == nil || len(links) == 0 {
		return false
	}
	// §6: revoking a token id invalidates the link that carries it
	// and every link derived from it.
	for _, link := range links {
		if l.Has(link.Payload.TokenID) {
			return true
		}
	}
	return false
}
