package cache

import (
	"testing"

	"verbgate/internal/authz"
)

func review() authz.SubjectAccessReview {
	return authz.SubjectAccessReview{
		User:      "alice",
		Groups:    []string{"sre"},
		Verb:      "get",
		APIGroup:  "",
		Resource:  "pods",
		Namespace: "data",
		Name:      "db-prod",
	}
}

func TestLookupMissesBeforeStore(t *testing.T) {
	c := New(4)
	if _, _, ok := c.Lookup("v1", review()); ok {
		t.Fatalf("expected lookup miss on empty cache")
	}
}

func TestStoreThenLookupHits(t *testing.T) {
	c := New(4)
	req := review()
	c.Store("v1", req, authz.DecisionAllow, "rule allows get on pods")
	decision, reason, ok := c.Lookup("v1", req)
	if !ok {
		t.Fatalf("expected cache hit")
	}
	if decision != authz.DecisionAllow {
		t.Fatalf("decision = %s, want allow", decision)
	}
	if reason == "" {
		t.Fatalf("expected reason to be preserved")
	}
}

func TestEvictionKeepsCapacity(t *testing.T) {
	c := New(2)
	for _, name := range []string{"a", "b", "c"} {
		req := review()
		req.Name = name
		c.Store("v1", req, authz.DecisionAllow, "")
	}
	if got := c.Len(); got != 2 {
		t.Fatalf("cache length = %d, want 2", got)
	}
}

func TestResetEmptiesCache(t *testing.T) {
	c := New(4)
	req := review()
	c.Store("v1", req, authz.DecisionAllow, "")
	c.Reset()
	if _, _, ok := c.Lookup("v1", req); ok {
		t.Fatalf("expected lookup miss after reset")
	}
}
