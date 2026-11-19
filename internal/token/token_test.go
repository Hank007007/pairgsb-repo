package token

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestMintAndUnwrap(t *testing.T) {
	m := NewManager()
	w, err := m.Mint("svc-payments", "kv/app/payments/stripe", 0)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if !strings.HasPrefix(w.Value, "wrap_") {
		t.Fatalf("wrap token %q lacks the expected prefix", w.Value)
	}
	if w.Path != "kv/app/payments/stripe" || w.Accessor != "svc-payments" {
		t.Fatalf("minted wrap has wrong metadata: %+v", w)
	}

	got, err := m.Unwrap(w.Value)
	if err != nil {
		t.Fatalf("Unwrap: %v", err)
	}
	if got.Value != w.Value || got.Path != w.Path || got.Accessor != w.Accessor {
		t.Fatalf("Unwrap returned %+v, want %+v", got, w)
	}
}

func TestUnwrapUnknownToken(t *testing.T) {
	m := NewManager()
	if _, err := m.Unwrap("wrap_does_not_exist"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Unwrap unknown = %v, want ErrNotFound", err)
	}
}

func TestMintDefaultTTL(t *testing.T) {
	m := NewManager()
	before := time.Now()
	w, err := m.Mint("svc-a", "kv/app/x", 0)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if w.ExpiresAt.IsZero() {
		t.Fatal("expected a non-zero expiry for the default TTL")
	}
	if w.ExpiresAt.Before(before) {
		t.Fatalf("expiry %v is before mint time %v", w.ExpiresAt, before)
	}
}

func TestMintNegativeTTLUsesDefault(t *testing.T) {
	m := NewManager()
	w, err := m.Mint("svc-a", "kv/app/x", -5)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if w.ExpiresAt.IsZero() {
		t.Fatal("expected a non-zero expiry")
	}
}

func TestCloseDropsTokens(t *testing.T) {
	m := NewManager()
	w, err := m.Mint("svc-a", "kv/app/x", 0)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	m.Close()
	if _, err := m.Unwrap(w.Value); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Unwrap after Close = %v, want ErrNotFound", err)
	}
}
