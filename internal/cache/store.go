package cache

import (
	"sync"
	"time"
)

// Effect mirrors the policy effect attached to a cached decision.
type Effect string

const (
	Allow Effect = "allow"
	Deny  Effect = "deny"
)

// Entry is one cached decision together with the metadata needed to report
// where it came from and when it stops being usable.
type Entry struct {
	Key         Key
	Tenant      string
	Subject     string
	Action      string
	Resource    string
	Effect      Effect
	MatchedRule string
	Revision    string
	ExpiresAt   time.Time
}

// Expired reports whether the entry is unusable at now.
func (e Entry) Expired(now time.Time) bool {
	return !e.ExpiresAt.IsZero() && !now.Before(e.ExpiresAt)
}

// Store is a concurrency-safe map from Key to Entry with counters for
// observable hit and miss behaviour.
type Store struct {
	mu      sync.RWMutex
	entries map[Key]Entry
	hits    uint64
	misses  uint64
	evicted uint64
}

// NewStore returns an empty Store.
func NewStore() *Store {
	return &Store{entries: make(map[Key]Entry)}
}

// Get returns the entry for k when it exists and has not expired.
func (s *Store) Get(k Key, now time.Time) (Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[k]
	if !ok {
		s.misses++
		return Entry{}, false
	}
	if e.Expired(now) {
		delete(s.entries, k)
		s.misses++
		s.evicted++
		return Entry{}, false
	}
	s.hits++
	return e, true
}

// Put inserts or replaces an entry.
func (s *Store) Put(e Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[e.Key] = e
}

// Delete removes the entry for k and reports whether it was present.
func (s *Store) Delete(k Key) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.entries[k]; ok {
		delete(s.entries, k)
		s.evicted++
		return true
	}
	return false
}

// RemoveWhere deletes every entry for which keep reports false and returns
// the number removed.
func (s *Store) RemoveWhere(keep func(Entry) bool) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for k, e := range s.entries {
		if !keep(e) {
			delete(s.entries, k)
			n++
		}
	}
	s.evicted += uint64(n)
	return n
}

// PurgeRevision deletes every entry written under revision.
func (s *Store) PurgeRevision(revision string) int {
	return s.RemoveWhere(func(e Entry) bool { return e.Revision != revision })
}

// PurgeExpired deletes entries that are no longer usable at now.
func (s *Store) PurgeExpired(now time.Time) int {
	return s.RemoveWhere(func(e Entry) bool { return !e.Expired(now) })
}

// Len returns the number of stored entries, including expired ones that have
// not yet been collected.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// Stats is a snapshot of the store counters.
type Stats struct {
	Entries   int
	Hits      uint64
	Misses    uint64
	Evictions uint64
	HitRatio  float64
}

// Stats returns a snapshot of the store counters. HitRatio is the fraction of
// Get calls that were served from cache, or zero when there were none.
func (s *Store) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := Stats{
		Entries:   len(s.entries),
		Hits:      s.hits,
		Misses:    s.misses,
		Evictions: s.evicted,
	}
	if total := st.Hits + st.Misses; total > 0 {
		st.HitRatio = float64(st.Hits) / float64(total)
	}
	return st
}
