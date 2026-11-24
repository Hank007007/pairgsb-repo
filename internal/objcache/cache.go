// Package objcache keeps recently fetched objects in memory so that repeated
// downloads of the same object do not go back to the upstream store.
package objcache

import (
	"path"
	"sync"
	"time"
)

// Object is one object as it was seen upstream, together with the metadata the
// edge needs in order to answer conditional requests and build reply headers.
type Object struct {
	Tenant       string
	Path         string
	ETag         string
	LastModified time.Time
	ContentType  string
	Filename     string
	CacheControl string

	// Status is the upstream status code this object was read from.
	Status int

	Size int64
	Body []byte
}

type entry struct {
	obj     *Object
	expires time.Time
	used    int64
}

// Cache is a bounded, process local store of objects.
type Cache struct {
	mu         sync.Mutex
	maxEntries int
	ttl        time.Duration
	entries    map[string]*entry
	current    map[string]string
	tick       int64
}

// New returns a cache that holds at most maxEntries objects and forgets an
// entry ttl after it was stored. A ttl of zero keeps entries until they are
// evicted.
func New(maxEntries int, ttl time.Duration) *Cache {
	if maxEntries <= 0 {
		maxEntries = 1
	}
	return &Cache{
		maxEntries: maxEntries,
		ttl:        ttl,
		entries:    make(map[string]*entry),
		current:    make(map[string]string),
	}
}

// Key builds the cache key for one object version. The tenant, the cleaned
// object path and the object's ETag are joined with a separator none of them
// can carry, so two distinct triples never produce the same key.
func (c *Cache) Key(tenant, objectPath, etag string) string {
	return tenant + "\x00" + path.Clean("/"+objectPath) + "\x00" + etag
}

// scope identifies one object across versions: everything in the key except
// the ETag.
func scope(tenant, objectPath string) string {
	return tenant + "\x00" + path.Clean("/"+objectPath)
}

// Get returns the object stored under key while it is still fresh.
func (c *Cache) Get(key string) (*Object, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if c.ttl > 0 && time.Now().After(e.expires) {
		delete(c.entries, key)
		c.dropCurrentLocked(key)
		return nil, false
	}
	c.tick++
	e.used = c.tick
	return e.obj, true
}

// Current returns the key and the object of the live entry for tenant and
// objectPath, whatever version it holds. Callers use it to find the entry a
// new request should be checked against before they know the current ETag.
func (c *Cache) Current(tenant, objectPath string) (string, *Object, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	s := scope(tenant, objectPath)
	key, ok := c.current[s]
	if !ok {
		return "", nil, false
	}
	e, ok := c.entries[key]
	if !ok || (c.ttl > 0 && time.Now().After(e.expires)) {
		delete(c.entries, key)
		delete(c.current, s)
		return "", nil, false
	}
	c.tick++
	e.used = c.tick
	return key, e.obj, true
}

// Put stores obj under key, dropping the least recently used entry when the
// cache is over capacity. The entry becomes the current one for its tenant
// and path; any older version of the same object is forgotten.
func (c *Cache) Put(key string, obj *Object) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.tick++
	c.entries[key] = &entry{
		obj:     obj,
		expires: time.Now().Add(c.ttl),
		used:    c.tick,
	}

	s := scope(obj.Tenant, obj.Path)
	if old, ok := c.current[s]; ok && old != key {
		delete(c.entries, old)
	}
	c.current[s] = key

	for len(c.entries) > c.maxEntries {
		c.evictOldestLocked()
	}
}

func (c *Cache) evictOldestLocked() {
	var oldestKey string
	var oldest int64
	first := true
	for key, e := range c.entries {
		if first || e.used < oldest {
			oldestKey, oldest, first = key, e.used, false
		}
	}
	if first {
		return
	}
	delete(c.entries, oldestKey)
	c.dropCurrentLocked(oldestKey)
}

// dropCurrentLocked removes key from the current-version index if it is
// indexed there.
func (c *Cache) dropCurrentLocked(key string) {
	for s, k := range c.current {
		if k == key {
			delete(c.current, s)
			return
		}
	}
}

// Len reports how many entries are stored.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}
