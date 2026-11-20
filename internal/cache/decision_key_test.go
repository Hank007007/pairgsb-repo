package cache

import (
	"testing"

	"verbgate/internal/authz"
)

func TestCacheSeparatesUsers(t *testing.T) {
	c := New(8)
	alice := review()
	bob := review()
	bob.User = "bob"

	c.Store("v1", alice, authz.DecisionAllow, "")
	if _, _, ok := c.Lookup("v1", bob); ok {
		t.Fatalf("bob must not reuse alice's cached decision")
	}
}

func TestCacheSeparatesGroupSets(t *testing.T) {
	c := New(8)
	base := review()
	other := review()
	other.Groups = []string{"db-oncall"}

	c.Store("v1", base, authz.DecisionAllow, "")
	if _, _, ok := c.Lookup("v1", other); ok {
		t.Fatalf("different group sets must not share cache entries")
	}
}

func TestCacheSeparatesImpersonation(t *testing.T) {
	c := New(8)
	direct := review()
	impersonated := review()
	impersonated.Impersonation = &authz.Impersonation{User: "bob"}

	c.Store("v1", direct, authz.DecisionAllow, "")
	if _, _, ok := c.Lookup("v1", impersonated); ok {
		t.Fatalf("impersonated request must not reuse the direct identity entry")
	}

	otherImpersonation := review()
	otherImpersonation.Impersonation = &authz.Impersonation{User: "carol"}
	c.Store("v1", impersonated, authz.DecisionAllow, "")
	if _, _, ok := c.Lookup("v1", otherImpersonation); ok {
		t.Fatalf("different impersonated users must not share cache entries")
	}
}

func TestCacheSeparatesSubresource(t *testing.T) {
	c := New(8)
	plain := review()
	exec := review()
	exec.Subresource = "exec"
	exec.Name = ""

	c.Store("v1", plain, authz.DecisionAllow, "")
	if _, _, ok := c.Lookup("v1", exec); ok {
		t.Fatalf("subresource request must not reuse the plain resource entry")
	}
}

func TestCacheKeyIncludesSnapshotVersion(t *testing.T) {
	c := New(8)
	req := review()
	c.Store("v1", req, authz.DecisionAllow, "")
	if _, _, ok := c.Lookup("v2", req); ok {
		t.Fatalf("new snapshot version must not reuse decisions from the old version")
	}
}
