package resolve

import (
	"sync"
	"time"
)

type cacheEntry struct {
	value   string
	expires time.Time
}

// Cache is a small TTL cache in front of provider lookups.
type Cache struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	entries map[string]cacheEntry
}

// NewCache returns a cache whose entries live for ttl.
func NewCache(ttl time.Duration) *Cache {
	return NewCacheWithClock(ttl, time.Now)
}

// NewCacheWithClock is NewCache with an injectable clock, for tests.
func NewCacheWithClock(ttl time.Duration, now func() time.Time) *Cache {
	if now == nil {
		now = time.Now
	}
	return &Cache{
		ttl:     ttl,
		now:     now,
		entries: make(map[string]cacheEntry),
	}
}

// Get returns the cached value for key, or calls fetch and stores the result.
func (c *Cache) Get(key string, fetch func() (string, error)) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.entries[key]; ok {
		if entry.expires.IsZero() || c.now().Before(entry.expires) {
			return entry.value, nil
		}
	}

	value, err := fetch()
	entry := cacheEntry{value: value}
	if err == nil {
		entry.expires = c.now().Add(c.ttl)
	}
	c.entries[key] = entry
	return value, err
}

// Delete removes key from the cache.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// Len reports the number of cached entries.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}
