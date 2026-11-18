package policy

import (
	"testing"
	"time"
)

var testClock = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func TestMatchSubject(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{"user:alice", "user:alice", true},
		{"user:alice", "user:bob", false},
		{"user:*", "user:alice", true},
		{"user:*", "user:team:core", true},
		{"*", "anything", true},
		{"doc:*:public", "doc:42:public", true},
		{"doc:*:public", "doc:42:private", false},
		{"service.*", "service.api", true},
		{"service.*", "service", false},
	}

	for _, tt := range tests {
		if got := matchSubject(tt.pattern, tt.value); got != tt.want {
			t.Errorf("matchSubject(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
		}
	}
}

func TestMatchActionSingleSegment(t *testing.T) {
	tests := []struct {
		pattern string
		action  string
		want    bool
	}{
		{"read", "read", true},
		{"read", "delete", false},
		{"*", "read", true},
		{"docs.read", "docs.read", true},
		{"docs.read", "docs.write", false},
	}

	for _, tt := range tests {
		if got := matchAction(tt.pattern, tt.action); got != tt.want {
			t.Errorf("matchAction(%q, %q) = %v, want %v", tt.pattern, tt.action, got, tt.want)
		}
	}
}

func TestInWindow(t *testing.T) {
	r := Rule{
		NotBefore: testClock.Add(-time.Hour),
		NotAfter:  testClock.Add(time.Hour),
	}
	if !inWindow(r, testClock) {
		t.Error("rule should be valid inside its window")
	}
	if inWindow(r, testClock.Add(-2*time.Hour)) {
		t.Error("rule should not be valid before NotBefore")
	}
	if inWindow(r, testClock.Add(2*time.Hour)) {
		t.Error("rule should not be valid after NotAfter")
	}
	if inWindow(r, testClock.Add(time.Hour)) {
		t.Error("NotAfter is an exclusive bound")
	}

	if !inWindow(Rule{}, testClock) {
		t.Error("zero window should be unbounded")
	}
}

func TestMatchesCombinesDimensions(t *testing.T) {
	r := Rule{
		ID:       "allow-alice",
		Effect:   Allow,
		Tenant:   "tenant-a",
		Subject:  "user:alice",
		Action:   "read",
		Resource: "doc:1",
	}

	if !Matches(r, "tenant-a", "user:alice", "read", "doc:1", testClock) {
		t.Fatal("expected the rule to match")
	}
	if Matches(r, "tenant-b", "user:alice", "read", "doc:1", testClock) {
		t.Fatal("matched a request from another tenant")
	}
	if Matches(r, "tenant-a", "user:bob", "read", "doc:1", testClock) {
		t.Fatal("matched a different subject")
	}
	if Matches(r, "tenant-a", "user:alice", "write", "doc:1", testClock) {
		t.Fatal("matched a different action")
	}
	if Matches(r, "tenant-a", "user:alice", "read", "doc:2", testClock) {
		t.Fatal("matched a different resource")
	}
}

func TestWildcardTenantRule(t *testing.T) {
	r := Rule{
		ID:       "allow-any-tenant",
		Effect:   Allow,
		Tenant:   "*",
		Subject:  "user:*",
		Action:   "read",
		Resource: "doc:*",
	}
	for _, tenant := range []string{"tenant-a", "tenant-b"} {
		if !Matches(r, tenant, "user:alice", "read", "doc:1", testClock) {
			t.Errorf("wildcard tenant rule did not match tenant %q", tenant)
		}
	}
}

func TestEvaluateDenyWins(t *testing.T) {
	b := &Bundle{
		Revision: "1",
		Rules: []Rule{
			{ID: "allow-read", Effect: Allow, Tenant: "t", Subject: "user:alice", Action: "read", Resource: "doc:1"},
			{ID: "deny-secret", Effect: Deny, Tenant: "t", Subject: "user:alice", Action: "read", Resource: "doc:1"},
		},
	}
	effect, rule := Evaluate(b, "t", "user:alice", "read", "doc:1", testClock)
	if effect != Deny || rule != "deny-secret" {
		t.Fatalf("Evaluate = %v, %q; want deny, deny-secret", effect, rule)
	}
}

func TestEvaluateDefaultAllow(t *testing.T) {
	b := &Bundle{Revision: "1"}
	effect, rule := Evaluate(b, "t", "user:alice", "read", "doc:9", testClock)
	if effect != Allow || rule != "" {
		t.Fatalf("Evaluate = %v, %q; want allow, empty", effect, rule)
	}
}

func TestEvaluateSkipsRulesOutsideWindow(t *testing.T) {
	b := &Bundle{
		Revision: "1",
		Rules: []Rule{
			{
				ID: "deny-expired", Effect: Deny, Tenant: "t", Subject: "user:*",
				Action: "read", Resource: "doc:*",
				NotBefore: testClock.Add(-2 * time.Hour),
				NotAfter:  testClock.Add(-time.Hour),
			},
		},
	}
	effect, rule := Evaluate(b, "t", "user:alice", "read", "doc:1", testClock)
	if effect != Allow || rule != "" {
		t.Fatalf("Evaluate = %v, %q; want allow, empty", effect, rule)
	}
}

func TestNextBoundary(t *testing.T) {
	later := testClock.Add(30 * time.Minute)
	sooner := testClock.Add(10 * time.Minute)
	rules := []Rule{
		{NotAfter: later},
		{NotAfter: sooner},
		{NotBefore: testClock.Add(-time.Hour)},
	}
	got, ok := NextBoundary(rules, testClock)
	if !ok || !got.Equal(sooner) {
		t.Fatalf("NextBoundary = %v, %v; want %v, true", got, ok, sooner)
	}

	if _, ok := NextBoundary([]Rule{{}}, testClock); ok {
		t.Fatal("NextBoundary found a boundary on an unbounded rule set")
	}
}
