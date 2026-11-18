package cache

import (
	"testing"
	"time"
)

var storeClock = time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)

func testEntry(key Key, rev string, expires time.Time) Entry {
	return Entry{
		Key:       key,
		Tenant:    "tenant-a",
		Subject:   "user:alice",
		Action:    "read",
		Resource:  "doc:1",
		Effect:    Allow,
		Revision:  rev,
		ExpiresAt: expires,
	}
}

func TestStorePutGet(t *testing.T) {
	s := NewStore()
	e := testEntry("k1", "1", storeClock.Add(time.Minute))

	if _, ok := s.Get("k1", storeClock); ok {
		t.Fatal("empty store returned an entry")
	}

	s.Put(e)
	got, ok := s.Get("k1", storeClock)
	if !ok {
		t.Fatal("stored entry was not found")
	}
	if got.Effect != Allow || got.Revision != "1" {
		t.Fatalf("Get = %+v", got)
	}

	stats := s.Stats()
	if stats.Hits != 1 || stats.Misses != 1 {
		t.Fatalf("Stats = %+v; want 1 hit and 1 miss", stats)
	}
}

func TestStoreExpiry(t *testing.T) {
	s := NewStore()
	s.Put(testEntry("k1", "1", storeClock.Add(time.Minute)))

	if _, ok := s.Get("k1", storeClock.Add(2*time.Minute)); ok {
		t.Fatal("expired entry was returned")
	}
	if s.Len() != 0 {
		t.Fatalf("Len = %d; expired entry should be removed", s.Len())
	}
}

func TestStoreDelete(t *testing.T) {
	s := NewStore()
	s.Put(testEntry("k1", "1", storeClock.Add(time.Minute)))

	if !s.Delete("k1") {
		t.Fatal("Delete reported a missing entry")
	}
	if s.Delete("k1") {
		t.Fatal("Delete removed the same entry twice")
	}
	if s.Len() != 0 {
		t.Fatalf("Len = %d, want 0", s.Len())
	}
}

func TestStoreRemoveWhere(t *testing.T) {
	s := NewStore()
	s.Put(testEntry("k1", "1", storeClock.Add(time.Minute)))
	s.Put(testEntry("k2", "1", storeClock.Add(time.Minute)))
	s.Put(testEntry("k3", "2", storeClock.Add(time.Minute)))

	removed := s.RemoveWhere(func(e Entry) bool { return e.Key == "k2" })
	if removed != 2 {
		t.Fatalf("RemoveWhere removed %d, want 2", removed)
	}
	if s.Len() != 1 {
		t.Fatalf("Len = %d, want 1", s.Len())
	}
	if _, ok := s.Get("k2", storeClock); !ok {
		t.Fatal("the kept entry was removed")
	}
}

func TestStorePurgeRevision(t *testing.T) {
	s := NewStore()
	s.Put(testEntry("old", "1", storeClock.Add(time.Minute)))
	s.Put(testEntry("new", "2", storeClock.Add(time.Minute)))

	if removed := s.PurgeRevision("1"); removed != 1 {
		t.Fatalf("PurgeRevision removed %d, want 1", removed)
	}
	if removed := s.PurgeRevision("2"); removed != 1 {
		t.Fatalf("PurgeRevision removed %d, want 1", removed)
	}
	if s.Len() != 0 {
		t.Fatalf("Len = %d, want 0", s.Len())
	}
}

func TestStorePurgeExpired(t *testing.T) {
	s := NewStore()
	s.Put(testEntry("gone", "1", storeClock.Add(time.Minute)))
	s.Put(testEntry("live", "1", storeClock.Add(time.Hour)))

	if removed := s.PurgeExpired(storeClock.Add(10 * time.Minute)); removed != 1 {
		t.Fatalf("PurgeExpired removed %d, want 1", removed)
	}
	if _, ok := s.Get("live", storeClock); !ok {
		t.Fatal("PurgeExpired removed a live entry")
	}
}

func TestStatsHitRatio(t *testing.T) {
	s := NewStore()
	s.Put(testEntry("k1", "1", storeClock.Add(time.Minute)))

	if ratio := s.Stats().HitRatio; ratio != 0 {
		t.Fatalf("HitRatio on a fresh store = %v, want 0", ratio)
	}
	s.Get("k1", storeClock)
	s.Get("k1", storeClock)
	s.Get("missing", storeClock)

	stats := s.Stats()
	if stats.Hits != 2 || stats.Misses != 1 {
		t.Fatalf("Stats = %+v", stats)
	}
	if stats.HitRatio < 0.66 || stats.HitRatio > 0.67 {
		t.Fatalf("HitRatio = %v, want about 0.667", stats.HitRatio)
	}
}
