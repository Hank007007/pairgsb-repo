package cache

import (
	"testing"
	"time"

	"scopewall/internal/policy"
)

func testKey(subject string) Key {
	return NewKey(policy.Request{
		Subject:  policy.Subject{ServiceAccount: subject, Roles: []string{"reader"}, RoleVersion: "1"},
		Resource: "/orgs/acme/projects/42",
		Action:   "read",
	})
}

func TestCacheHit(t *testing.T) {
	c := New(16, time.Minute)
	key := testKey("svc-a")
	want := policy.Decision{Effect: policy.Allow, Reason: "rule \"reader\"", Source: policy.SourcePolicy}

	c.Put(key, want)
	got, ok := c.Get(key)
	if !ok {
		t.Fatal("expected a cache hit")
	}
	if got.Effect != want.Effect {
		t.Fatalf("effect = %q, want %q", got.Effect, want.Effect)
	}
	if got.Source != policy.SourceCache {
		t.Fatalf("source = %q, want %q", got.Source, policy.SourceCache)
	}
}

func TestCacheMissForDifferentSubject(t *testing.T) {
	c := New(16, time.Minute)
	c.Put(testKey("svc-a"), policy.Decision{Effect: policy.Allow})
	if _, ok := c.Get(testKey("svc-b")); ok {
		t.Fatal("expected a miss for a different subject")
	}
}

func TestCacheExpiresAfterTTL(t *testing.T) {
	c := New(16, time.Minute)
	now := time.Unix(1000, 0)
	c.now = func() time.Time { return now }

	key := testKey("svc-a")
	c.Put(key, policy.Decision{Effect: policy.Allow})
	if _, ok := c.Get(key); !ok {
		t.Fatal("expected a hit before expiry")
	}

	now = now.Add(2 * time.Minute)
	if _, ok := c.Get(key); ok {
		t.Fatal("expected a miss after the TTL")
	}
}

func TestCacheEvictsLeastRecentlyUsed(t *testing.T) {
	c := New(2, time.Minute)
	a, b, d := testKey("svc-a"), testKey("svc-b"), testKey("svc-c")

	c.Put(a, policy.Decision{Effect: policy.Allow})
	c.Put(b, policy.Decision{Effect: policy.Allow})
	if _, ok := c.Get(a); !ok {
		t.Fatal("expected a hit for the first key")
	}

	c.Put(d, policy.Decision{Effect: policy.Allow})
	if _, ok := c.Get(b); ok {
		t.Fatal("expected the least recently used key to be evicted")
	}
	if _, ok := c.Get(a); !ok {
		t.Fatal("expected the recently used key to survive")
	}
}
