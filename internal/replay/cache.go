// Package replay remembers accepted DPoP proofs so that each one is honoured
// at most once within its validity window.
package replay

import (
	"sync"
	"time"
)

// Cache is a bounded in-memory set of accepted proofs. A record keeps the
// time at which the proof's retention window ends.
type Cache struct {
	mu      sync.Mutex
	entries map[string]time.Time
	ttl     time.Duration
}

// NewCache returns a cache configured with the given retention window.
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		entries: make(map[string]time.Time),
		ttl:     ttl,
	}
}

// MarkSeen records that a proof was accepted and reports whether it had
// already been seen. ttl is the retention the caller asks for this record.
// The check-and-record is atomic, so concurrent arrivals of the same proof
// cannot both be accepted. Uniqueness is scoped to (jkt, jti).
func (c *Cache) MarkSeen(jkt, jti string, now time.Time, ttl time.Duration) bool {
	if ttl <= 0 {
		ttl = c.ttl
	}
	id := jkt + "\x00" + jti

	c.mu.Lock()
	defer c.mu.Unlock()

	c.prune(now)
	if _, ok := c.entries[id]; ok {
		return true
	}
	c.entries[id] = now.Add(ttl)
	return false
}

// Get reports whether id is currently recorded. The cache is tidied up as part
// of the lookup.
func (c *Cache) Get(id string, now time.Time) (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.prune(now)
	stamp, ok := c.entries[id]
	return stamp, ok
}

// Prune drops records whose retention window has ended.
func (c *Cache) Prune(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prune(now)
}

func (c *Cache) prune(now time.Time) {
	for id, stamp := range c.entries {
		if !stamp.After(now) {
			delete(c.entries, id)
		}
	}
}

// Len returns the number of recorded identifiers.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}
