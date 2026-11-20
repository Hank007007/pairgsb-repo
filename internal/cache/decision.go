package cache

import (
	"container/list"
	"strings"
	"sync"

	"verbgate/internal/authz"
)

// Cache stores recent decisions with a bounded number of entries. When the
// capacity is reached the oldest entry is evicted.
type Cache struct {
	mu       sync.Mutex
	capacity int
	order    *list.List
	entries  map[string]*list.Element
}

type entry struct {
	key      string
	decision authz.Decision
	reason   string
}

// New returns a cache holding at most capacity decisions. A capacity below one
// is treated as one.
func New(capacity int) *Cache {
	if capacity < 1 {
		capacity = 1
	}
	return &Cache{
		capacity: capacity,
		order:    list.New(),
		entries:  make(map[string]*list.Element, capacity),
	}
}

// Lookup returns a previously stored decision for the review.
func (c *Cache) Lookup(snapshotVersion string, review authz.SubjectAccessReview) (authz.Decision, string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := keyFor(snapshotVersion, review)
	element, ok := c.entries[key]
	if !ok {
		return authz.DecisionNoOpinion, "", false
	}
	c.order.MoveToFront(element)
	stored := element.Value.(*entry)
	return stored.decision, stored.reason, true
}

// Store records the decision for the review.
func (c *Cache) Store(snapshotVersion string, review authz.SubjectAccessReview, decision authz.Decision, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := keyFor(snapshotVersion, review)
	if element, ok := c.entries[key]; ok {
		element.Value.(*entry).decision = decision
		element.Value.(*entry).reason = reason
		c.order.MoveToFront(element)
		return
	}
	element := c.order.PushFront(&entry{key: key, decision: decision, reason: reason})
	c.entries[key] = element
	for c.order.Len() > c.capacity {
		oldest := c.order.Back()
		if oldest == nil {
			break
		}
		c.order.Remove(oldest)
		delete(c.entries, oldest.Value.(*entry).key)
	}
}

// Reset drops every stored decision.
func (c *Cache) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.order.Init()
	c.entries = make(map[string]*list.Element, c.capacity)
}

// Len reports how many decisions are currently cached.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// keyFor builds the lookup key for a review.
func keyFor(snapshotVersion string, review authz.SubjectAccessReview) string {
	impersonateUser := ""
	impersonateGroups := ""
	if review.Impersonation != nil {
		impersonateUser = review.Impersonation.User
		impersonateGroups = strings.Join(review.Impersonation.Groups, "\x01")
	}
	return strings.Join([]string{
		snapshotVersion,
		review.User,
		strings.Join(review.Groups, "\x01"),
		impersonateUser,
		impersonateGroups,
		review.Verb,
		review.APIGroup,
		review.Resource,
		review.Subresource,
		review.Namespace,
		review.Name,
	}, "\x00")
}
