package decide

import (
	"errors"
	"sync"
	"testing"
	"time"

	"permcache/internal/cache"
	"permcache/internal/policy"
)

var engineClock = time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)

func newTestEngine(t *testing.T, b policy.Bundle) *Engine {
	t.Helper()

	l := policy.NewLoader()
	if err := l.Load(b); err != nil {
		t.Fatalf("Load: %v", err)
	}
	e := New(l)
	e.SetClock(func() time.Time { return engineClock })
	return e
}

func singleRuleBundle(effect policy.Effect) policy.Bundle {
	return policy.Bundle{
		Revision: "1",
		Rules: []policy.Rule{
			{
				ID:       "rule-1",
				Effect:   effect,
				Tenant:   "tenant-a",
				Subject:  "user:alice",
				Action:   "read",
				Resource: "doc:1",
			},
		},
	}
}

func aliceReadsDocOne() Request {
	return Request{
		Tenant:   "tenant-a",
		Subject:  "user:alice",
		Action:   "read",
		Resource: "doc:1",
	}
}

func TestDecideAllowThenCacheHit(t *testing.T) {
	e := newTestEngine(t, singleRuleBundle(policy.Allow))

	first, err := e.Decide(aliceReadsDocOne())
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if first.Effect != cache.Allow || first.MatchedRule != "rule-1" || first.Cached {
		t.Fatalf("first Decide = %+v", first)
	}

	second, err := e.Decide(aliceReadsDocOne())
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if second.Effect != cache.Allow || !second.Cached {
		t.Fatalf("second Decide = %+v", second)
	}

	stats := e.Store().Stats()
	if stats.Hits != 1 || stats.Misses != 1 {
		t.Fatalf("Stats = %+v; want 1 hit and 1 miss", stats)
	}
}

func TestDecideDenyIsCached(t *testing.T) {
	e := newTestEngine(t, singleRuleBundle(policy.Deny))

	first, err := e.Decide(aliceReadsDocOne())
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	second, err := e.Decide(aliceReadsDocOne())
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}

	if first.Effect != cache.Deny || second.Effect != cache.Deny {
		t.Fatalf("effects = %v, %v; want deny, deny", first.Effect, second.Effect)
	}
	if !second.Cached {
		t.Fatal("second decision was not served from the cache")
	}
}

func TestDecideDefaultAllow(t *testing.T) {
	e := newTestEngine(t, singleRuleBundle(policy.Allow))

	d, err := e.Decide(Request{
		Tenant:   "tenant-a",
		Subject:  "user:alice",
		Action:   "read",
		Resource: "doc:2",
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if d.Effect != cache.Allow || d.MatchedRule != "" {
		t.Fatalf("Decide = %+v; want the default allow decision", d)
	}
}

func TestDecideCachesPerResource(t *testing.T) {
	e := newTestEngine(t, singleRuleBundle(policy.Deny))

	if _, err := e.Decide(aliceReadsDocOne()); err != nil {
		t.Fatalf("Decide: %v", err)
	}

	d, err := e.Decide(Request{
		Tenant:   "tenant-a",
		Subject:  "user:alice",
		Action:   "read",
		Resource: "doc:2",
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if d.Cached {
		t.Fatal("a different resource was served from the cache")
	}
	if d.Effect != cache.Allow {
		t.Fatalf("effect = %v; doc:2 is not covered by the rule", d.Effect)
	}
}

func TestDecideExpiresAfterTTL(t *testing.T) {
	e := newTestEngine(t, singleRuleBundle(policy.Allow))
	e.SetTTL(30 * time.Second)

	at := engineClock
	e.SetClock(func() time.Time { return at })

	if _, err := e.Decide(aliceReadsDocOne()); err != nil {
		t.Fatalf("Decide: %v", err)
	}

	at = at.Add(29 * time.Second)
	if d, err := e.Decide(aliceReadsDocOne()); err != nil || !d.Cached {
		t.Fatalf("Decide inside TTL = %+v, %v; want a cache hit", d, err)
	}

	at = at.Add(2 * time.Second)
	if d, err := e.Decide(aliceReadsDocOne()); err != nil || d.Cached {
		t.Fatalf("Decide after TTL = %+v, %v; want a miss", d, err)
	}
}

func TestDecideRejectsIncompleteRequest(t *testing.T) {
	e := newTestEngine(t, singleRuleBundle(policy.Allow))

	_, err := e.Decide(Request{Tenant: "tenant-a", Subject: "user:alice", Action: "read"})
	if !errors.Is(err, ErrEmptyField) {
		t.Fatalf("Decide = %v, want ErrEmptyField", err)
	}
}

func TestDecideWithoutBundle(t *testing.T) {
	e := New(policy.NewLoader())
	e.SetClock(func() time.Time { return engineClock })

	if _, err := e.Decide(aliceReadsDocOne()); !errors.Is(err, policy.ErrNoBundle) {
		t.Fatalf("Decide = %v, want ErrNoBundle", err)
	}
}

func TestDecideReportsRevision(t *testing.T) {
	e := newTestEngine(t, singleRuleBundle(policy.Allow))

	if _, ok := e.Revision(); ok {
		t.Fatal("fresh engine reports a revision")
	}

	d, err := e.Decide(aliceReadsDocOne())
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if d.Revision != "1" {
		t.Fatalf("Decision.Revision = %q, want 1", d.Revision)
	}
	if rev, ok := e.Revision(); !ok || rev != "1" {
		t.Fatalf("Revision = %q, %v; want 1, true", rev, ok)
	}
}

func TestDecideConcurrent(t *testing.T) {
	e := newTestEngine(t, singleRuleBundle(policy.Allow))

	const goroutines = 8
	const calls = 25

	var wg sync.WaitGroup
	errs := make(chan error, goroutines*calls)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < calls; j++ {
				d, err := e.Decide(aliceReadsDocOne())
				if err != nil {
					errs <- err
					return
				}
				if d.Effect != cache.Allow {
					errs <- errors.New("unexpected effect")
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	stats := e.Store().Stats()
	if stats.Hits+stats.Misses != goroutines*calls {
		t.Fatalf("Stats = %+v; want %d total lookups", stats, goroutines*calls)
	}
}
