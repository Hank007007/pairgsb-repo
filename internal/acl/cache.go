package acl

import (
	"sync"

	"topicwarden/internal/tenant"
)

// Compiled is a filter that has been expanded into representative topics and
// evaluated once.
type Compiled struct {
	Filter   string
	Tenant   string
	Version  uint64
	Topics   []string
	Verdicts []Verdict
}

// CompiledCache memoizes the evaluation of a filter's expansion.
type CompiledCache struct {
	mu      sync.Mutex
	entries map[compiledCacheKey]Compiled
}

type compiledCacheKey struct {
	filter    string
	tenant    string
	principal string
	version   uint64
}

// NewCompiledCache returns an empty cache.
func NewCompiledCache() *CompiledCache {
	return &CompiledCache{entries: make(map[compiledCacheKey]Compiled)}
}

// Compile expands filter, evaluates every topic of the expansion for
// principalClass, and returns the result.
func (c *CompiledCache) Compile(rs *RuleSet, tenantID, principalClass, filter string) Compiled {
	c.mu.Lock()
	defer c.mu.Unlock()

	if rs == nil {
		rs = &RuleSet{}
	}
	normalizedFilter := tenant.NormalizeTopic(filter)
	key := compiledCacheKey{
		filter:    normalizedFilter,
		tenant:    tenantID,
		principal: principalClass,
		version:   rs.Version,
	}
	if got, ok := c.entries[key]; ok {
		return got
	}

	topics := Expand(normalizedFilter)
	out := Compiled{
		Filter:   normalizedFilter,
		Tenant:   tenantID,
		Version:  rs.Version,
		Topics:   topics,
		Verdicts: make([]Verdict, 0, len(topics)),
	}
	for _, topic := range topics {
		out.Verdicts = append(out.Verdicts, Evaluate(rs, principalClass, ActionSubscribe, topic))
	}
	c.entries[key] = out
	return out
}

// Purge drops every entry.
func (c *CompiledCache) Purge() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[compiledCacheKey]Compiled)
}

// Len reports how many filters are cached.
func (c *CompiledCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}
