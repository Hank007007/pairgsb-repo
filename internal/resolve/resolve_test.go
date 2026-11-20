package resolve

import (
	"context"
	"testing"
	"time"
)

func TestParseRef(t *testing.T) {
	ref, err := ParseRef("secret://kv/billing/api_key")
	if err != nil {
		t.Fatalf("ParseRef: %v", err)
	}
	if ref.Scheme != SchemeKV || ref.Path != "billing/api_key" {
		t.Fatalf("unexpected ref: %+v", ref)
	}
	if got := ref.String(); got != "secret://kv/billing/api_key" {
		t.Fatalf("String() = %q", got)
	}

	if _, err := ParseRef("billing/api_key"); err == nil {
		t.Fatal("expected an error for a value that is not a reference")
	}
	if _, err := ParseRef("secret://kv"); err == nil {
		t.Fatal("expected an error for a reference without a path")
	}
}

func TestKVProviderResolve(t *testing.T) {
	kv := NewKVProviderFromMap(map[string]map[string]string{
		"default": {"billing/api_key": "default-key"},
		"staging": {"billing/api_key": "staging-key"},
	})
	ctx := context.Background()

	got, err := kv.Resolve(ctx, "staging", Ref{Scheme: SchemeKV, Path: "billing/api_key"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "staging-key" {
		t.Fatalf("staging value = %q", got)
	}

	got, err = kv.Resolve(ctx, "default", Ref{Scheme: SchemeKV, Path: "billing/api_key"})
	if err != nil {
		t.Fatalf("Resolve default: %v", err)
	}
	if got != "default-key" {
		t.Fatalf("default value = %q", got)
	}

	if _, err := kv.Resolve(ctx, "default", Ref{Scheme: SchemeKV, Path: "missing/key"}); err == nil {
		t.Fatal("expected an error for a missing kv key")
	}
}

func TestEnvProviderResolve(t *testing.T) {
	env := &EnvProvider{Lookup: func(name string) (string, bool) {
		if name == "DB_PASSWORD" {
			return "env-password", true
		}
		return "", false
	}}
	ctx := context.Background()

	got, err := env.Resolve(ctx, "default", Ref{Scheme: SchemeEnv, Path: "DB_PASSWORD"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "env-password" {
		t.Fatalf("value = %q", got)
	}
	if _, err := env.Resolve(ctx, "default", Ref{Scheme: SchemeEnv, Path: "MISSING"}); err == nil {
		t.Fatal("expected an error for an unset variable")
	}
}

func TestFileProviderResolve(t *testing.T) {
	file := NewFileProvider("../../testdata/secrets")
	ctx := context.Background()

	got, err := file.Resolve(ctx, "default", Ref{Scheme: SchemeFile, Path: "certs/tls.pem"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := "-----BEGIN CERTIFICATE-----\n" +
		"MIIBszCCAVmgAwIBAgIUY2VydGlmaWNhdGUtZm9yLXRlc3Rz\n" +
		"-----END CERTIFICATE-----"
	if got != want {
		t.Fatalf("value = %q", got)
	}
}

func TestCacheHitWithinTTL(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cache := NewCacheWithClock(time.Minute, func() time.Time { return now })

	calls := 0
	fetch := func() (string, error) {
		calls++
		return "value", nil
	}

	for i := 0; i < 3; i++ {
		got, err := cache.Get("secret://kv/billing/api_key", fetch)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got != "value" {
			t.Fatalf("value = %q", got)
		}
	}
	if calls != 1 {
		t.Fatalf("fetch called %d times, want 1", calls)
	}

	now = now.Add(2 * time.Minute)
	if _, err := cache.Get("secret://kv/billing/api_key", fetch); err != nil {
		t.Fatalf("Get after expiry: %v", err)
	}
	if calls != 2 {
		t.Fatalf("fetch called %d times after expiry, want 2", calls)
	}
}
