package cache

import (
	"container/list"
	"sync"
	"time"

	"scopewall/internal/policy"
)

// Key identifies a cached decision.
type Key struct {
	Subject  string
	Resource string
	Action   string
}

// NewKey derives the cache key for a request.
func NewKey(req policy.Request) Key {
	return Key{
		Subject:  req.Subject.ServiceAccount,
		Resource: req.Resource,
		Action:   req.Action,
	}
}

type entry struct {
	key     Key
	value   policy.Decision
	expires time.Time
}

// DecisionCache is a bounded LRU cache of decisions with a per-entry TTL.
type DecisionCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	maxSize int
	entries map[Key]*list.Element
	order   *list.List
	now     func() time.Time
}

// New builds a cache holding at most maxSize entries, each valid for ttl.
func New(maxSize int, ttl time.Duration) *DecisionCache {
	if maxSize <= 0 {
		maxSize = 1
	}
	return &DecisionCache{
		ttl:     ttl,
		maxSize: maxSize,
		entries: make(map[Key]*list.Element),
		order:   list.New(),
		now:     time.Now,
	}
}

// Get returns the cached decision for key when one is present and unexpired.
func (c *DecisionCache) Get(key Key) (policy.Decision, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.entries[key]
	if !ok {
		return policy.Decision{}, false
	}
	e := el.Value.(*entry)
	if !c.now().Before(e.expires) {
		c.removeElement(el)
		return policy.Decision{}, false
	}
	c.order.MoveToFront(el)
	value := e.value
	value.Source = policy.SourceCache
	return value, true
}

// Put stores a decision, evicting the least recently used entry when the cache is full.
func (c *DecisionCache) Put(key Key, value policy.Decision) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expires := c.now().Add(c.ttl)
	if el, ok := c.entries[key]; ok {
		e := el.Value.(*entry)
		e.value = value
		e.expires = expires
		c.order.MoveToFront(el)
		return
	}
	el := c.order.PushFront(&entry{key: key, value: value, expires: expires})
	c.entries[key] = el
	for c.order.Len() > c.maxSize {
		c.removeElement(c.order.Back())
	}
}

// Len reports the number of entries currently held.
func (c *DecisionCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

func (c *DecisionCache) removeElement(el *list.Element) {
	if el == nil {
		return
	}
	c.order.Remove(el)
	delete(c.entries, el.Value.(*entry).key)
}
