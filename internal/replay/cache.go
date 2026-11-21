// Package replay remembers accepted DPoP proofs so that each one is honoured
// at most once within its validity window.
package replay

import (
	"sync"
	"time"
)

// Cache is a bounded in-memory set of accepted proofs. A record keeps the
// second in which the proof was accepted.
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
func (c *Cache) MarkSeen(jkt, jti string, now time.Time, ttl time.Duration) bool {
	id := jti

	if _, ok := c.Get(id, now); ok {
		return true
	}

	if ttl <= 0 {
		ttl = c.ttl
	}

	c.mu.Lock()
	c.entries[id] = now.Truncate(time.Second)
	c.mu.Unlock()
	return false
}

// Get reports whether id is currently recorded. The cache is tidied up as part
// of the lookup.
func (c *Cache) Get(id string, now time.Time) (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Prune(now)
	stamp, ok := c.entries[id]
	return stamp, ok
}

// Prune drops records whose stamp is at or before the current second.
func (c *Cache) Prune(now time.Time) {
	for id, stamp := range c.entries {
		if stamp.Unix() <= now.Unix() {
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
