package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"scopewall/internal/cache"
	"scopewall/internal/policy"
)

func staticSource(p *policy.Policy) *Source {
	return NewSource(func() (*policy.Policy, error) { return p, nil }, 0)
}

func newTestAuthorizer(p *policy.Policy) *Authorizer {
	forwarded := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("upstream"))
	})
	return &Authorizer{
		Next:   forwarded,
		Source: staticSource(p),
		Cache:  cache.New(16, time.Minute),
	}
}

func TestMiddlewareForwardsAllowedRequest(t *testing.T) {
	p := &policy.Policy{Rules: []policy.Rule{{
		Name:     "platform-admin",
		Roles:    []string{"platform-admin"},
		Resource: "/**",
		Actions:  []string{"*"},
		Effect:   policy.Allow,
	}}}
	a := newTestAuthorizer(p)

	req := httptest.NewRequest(http.MethodGet, "/orgs/acme/projects/42", nil)
	req.Header.Set(HeaderServiceAccount, "svc-frontend")
	req.Header.Set(HeaderRoles, "platform-admin")
	req.Header.Set(HeaderRoleVersion, "3")
	rec := httptest.NewRecorder()

	a.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get(HeaderDecision); got != policy.SourcePolicy {
		t.Fatalf("decision source = %q, want %q", got, policy.SourcePolicy)
	}
}

func TestMiddlewareRejectsDeniedRequest(t *testing.T) {
	p := &policy.Policy{Rules: []policy.Rule{{
		Name:     "billing-frozen",
		Roles:    []string{"org-reader"},
		Resource: "/orgs/acme/billing/**",
		Actions:  []string{"*"},
		Effect:   policy.Deny,
	}}}
	a := newTestAuthorizer(p)

	req := httptest.NewRequest(http.MethodGet, "/orgs/acme/billing/invoice.pdf", nil)
	req.Header.Set(HeaderServiceAccount, "svc-billing")
	req.Header.Set(HeaderRoles, "org-reader")
	req.Header.Set(HeaderRoleVersion, "1")
	rec := httptest.NewRecorder()

	a.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if rec.Body.String() == "upstream" {
		t.Fatal("denied request reached the upstream handler")
	}
}

func TestMiddlewareServesSecondRequestFromCache(t *testing.T) {
	p := &policy.Policy{Rules: []policy.Rule{{
		Name:     "platform-admin",
		Roles:    []string{"platform-admin"},
		Resource: "/**",
		Actions:  []string{"*"},
		Effect:   policy.Allow,
	}}}
	a := newTestAuthorizer(p)

	build := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/orgs/acme/projects/42", nil)
		req.Header.Set(HeaderServiceAccount, "svc-frontend")
		req.Header.Set(HeaderRoles, "platform-admin")
		req.Header.Set(HeaderRoleVersion, "3")
		return req
	}

	first := httptest.NewRecorder()
	a.ServeHTTP(first, build())
	second := httptest.NewRecorder()
	a.ServeHTTP(second, build())

	if got := second.Header().Get(HeaderDecision); got != policy.SourceCache {
		t.Fatalf("decision source = %q, want %q", got, policy.SourceCache)
	}
	if second.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", second.Code, http.StatusOK)
	}
}

func TestSplitRoles(t *testing.T) {
	got := SplitRoles(" org-reader , auditor ,, ")
	want := []string{"org-reader", "auditor"}
	if len(got) != len(want) {
		t.Fatalf("roles = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("roles = %v, want %v", got, want)
		}
	}
}
