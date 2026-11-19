package lease

import (
	"errors"
	"testing"
)

func TestIssueAndCheck(t *testing.T) {
	m := NewManager()
	l := m.Issue("svc-payments", "kv/app/payments/stripe", "payments-core")
	if l.ID == "" {
		t.Fatal("expected a lease id")
	}
	got, err := m.Check(l.ID, "svc-payments", "kv/app/payments/stripe")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got.ID != l.ID || got.Backend != "payments-core" {
		t.Fatalf("Check returned %+v", got)
	}
}

func TestCheckRejectsOtherAccessorOrPath(t *testing.T) {
	m := NewManager()
	l := m.Issue("svc-payments", "kv/app/payments/stripe", "payments-core")

	if _, err := m.Check(l.ID, "svc-other", "kv/app/payments/stripe"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Check wrong accessor = %v, want ErrNotFound", err)
	}
	if _, err := m.Check(l.ID, "svc-payments", "kv/app/payments/refund"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Check wrong path = %v, want ErrNotFound", err)
	}
	if _, err := m.Check("lease_missing", "svc-payments", "kv/app/payments/stripe"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Check unknown id = %v, want ErrNotFound", err)
	}
}

func TestCacheValue(t *testing.T) {
	m := NewManager()
	if _, ok := m.CachedValue("payments-core", "kv/app/payments/stripe"); ok {
		t.Fatal("expected empty cache")
	}
	m.CacheValue("payments-core", "kv/app/payments/stripe", "sk_live_x")
	v, ok := m.CachedValue("payments-core", "kv/app/payments/stripe")
	if !ok || v != "sk_live_x" {
		t.Fatalf("CachedValue = %q, %v", v, ok)
	}
	if _, ok := m.CachedValue("payments-core", "kv/app/other"); ok {
		t.Fatal("expected cache miss for a different path")
	}
}

func TestRevokeSingleLease(t *testing.T) {
	m := NewManager()
	l := m.Issue("svc-payments", "kv/app/payments/stripe", "payments-core")

	n := m.Revoke("payments-core")
	if n != 1 {
		t.Fatalf("Revoke returned %d, want 1", n)
	}
	if _, err := m.Check(l.ID, "svc-payments", "kv/app/payments/stripe"); !errors.Is(err, ErrRevoked) {
		t.Fatalf("Check after revoke = %v, want ErrRevoked", err)
	}
}

func TestRevokeUnknownBackend(t *testing.T) {
	m := NewManager()
	if n := m.Revoke("no-such-backend"); n != 0 {
		t.Fatalf("Revoke unknown backend returned %d, want 0", n)
	}
}

func TestRevokeIdempotent(t *testing.T) {
	m := NewManager()
	l := m.Issue("svc-payments", "kv/app/payments/stripe", "payments-core")
	if n := m.Revoke("payments-core"); n != 1 {
		t.Fatalf("first Revoke returned %d, want 1", n)
	}
	if n := m.Revoke("payments-core"); n != 0 {
		t.Fatalf("second Revoke returned %d, want 0", n)
	}
	if _, err := m.Check(l.ID, "svc-payments", "kv/app/payments/stripe"); !errors.Is(err, ErrRevoked) {
		t.Fatalf("Check after revoke = %v, want ErrRevoked", err)
	}
}

func TestLeasesFor(t *testing.T) {
	m := NewManager()
	first := m.Issue("svc-a", "kv/app/a", "backend-x")
	second := m.Issue("svc-b", "kv/app/b", "backend-x")
	m.Issue("svc-c", "kv/app/c", "backend-y")

	ids := m.LeasesFor("backend-x")
	if len(ids) != 2 || ids[0] != first.ID || ids[1] != second.ID {
		t.Fatalf("LeasesFor = %v, want [%s %s]", ids, first.ID, second.ID)
	}
}
