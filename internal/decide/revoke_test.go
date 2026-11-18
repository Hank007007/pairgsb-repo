package decide

import (
	"errors"
	"testing"

	"permcache/internal/cache"
	"permcache/internal/policy"
)

func TestApplyRevocationRemovesExactEntry(t *testing.T) {
	e := newTestEngine(t, singleRuleBundle(policy.Deny))

	if _, err := e.Decide(aliceReadsDocOne()); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if e.Store().Len() != 1 {
		t.Fatalf("Len = %d, want 1", e.Store().Len())
	}

	removed, err := e.ApplyRevocation(Revocation{
		Tenant:   "tenant-a",
		Subject:  "user:alice",
		Action:   "read",
		Resource: "doc:1",
	})
	if err != nil {
		t.Fatalf("ApplyRevocation: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if e.Store().Len() != 0 {
		t.Fatalf("Len = %d, want 0", e.Store().Len())
	}

	again, err := e.Decide(aliceReadsDocOne())
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if again.Cached {
		t.Fatal("revoked entry was served from the cache")
	}
}

func TestApplyRevocationLeavesOtherEntries(t *testing.T) {
	e := newTestEngine(t, singleRuleBundle(policy.Allow))

	requests := []Request{
		aliceReadsDocOne(),
		{Tenant: "tenant-a", Subject: "user:alice", Action: "read", Resource: "doc:2"},
		{Tenant: "tenant-a", Subject: "user:bob", Action: "read", Resource: "doc:1"},
	}
	for _, req := range requests {
		if _, err := e.Decide(req); err != nil {
			t.Fatalf("Decide: %v", err)
		}
	}

	removed, err := e.ApplyRevocation(Revocation{
		Tenant:   "tenant-a",
		Subject:  "user:alice",
		Action:   "read",
		Resource: "doc:1",
	})
	if err != nil {
		t.Fatalf("ApplyRevocation: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if e.Store().Len() != 2 {
		t.Fatalf("Len = %d, want 2", e.Store().Len())
	}
}

func TestApplyRevocationValidatesFields(t *testing.T) {
	e := newTestEngine(t, singleRuleBundle(policy.Allow))

	_, err := e.ApplyRevocation(Revocation{Tenant: "tenant-a"})
	if !errors.Is(err, ErrInvalidRevocation) {
		t.Fatalf("ApplyRevocation = %v, want ErrInvalidRevocation", err)
	}
}

func TestApplyRevocationThenDecideReturnsFreshDecision(t *testing.T) {
	e := newTestEngine(t, singleRuleBundle(policy.Deny))

	first, err := e.Decide(aliceReadsDocOne())
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if first.Effect != cache.Deny {
		t.Fatalf("effect = %v, want deny", first.Effect)
	}

	if _, err := e.ApplyRevocation(Revocation{
		Tenant:   "tenant-a",
		Subject:  "user:alice",
		Action:   "read",
		Resource: "doc:1",
	}); err != nil {
		t.Fatalf("ApplyRevocation: %v", err)
	}

	second, err := e.Decide(aliceReadsDocOne())
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if second.Cached || second.Effect != cache.Deny {
		t.Fatalf("Decide after revocation = %+v; want a fresh deny", second)
	}
}
