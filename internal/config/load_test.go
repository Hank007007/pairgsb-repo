package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandVars(t *testing.T) {
	got, err := expandVars("invoice-${SERVICE_NAME}-${REGION}", map[string]string{
		"SERVICE_NAME": "checkout",
		"REGION":       "us-east-1",
	})
	if err != nil {
		t.Fatalf("expandVars: %v", err)
	}
	if want := "invoice-checkout-us-east-1"; got != want {
		t.Fatalf("expandVars = %q, want %q", got, want)
	}

	if _, err := expandVars("${UNKNOWN}", map[string]string{}); err == nil {
		t.Fatal("expected an error for an unknown placeholder")
	}
	if _, err := expandVars("${UNTERMINATED", map[string]string{}); err == nil {
		t.Fatal("expected an error for an unterminated placeholder")
	}
}

func TestLoadBaseDocumentWithKV(t *testing.T) {
	cfg := mustLoad(t, loadOptions(t, "../../testdata/base.json", "", "default"))

	if got, _ := cfg.Get("billing.api_key"); got != "kv-default-billing-key" {
		t.Fatalf("billing.api_key = %q", got)
	}
	if got, _ := cfg.Get("db.password"); got != "kv-default-billing-key" {
		t.Fatalf("db.password = %q", got)
	}
	if got, _ := cfg.Get("billing.publishable_key"); got != "pk-live-123" {
		t.Fatalf("billing.publishable_key = %q", got)
	}
	if got, _ := cfg.Get("service.name"); got != "checkout" {
		t.Fatalf("service.name = %q", got)
	}

	features, ok := cfg.List("features")
	if !ok {
		t.Fatal("features is not a list")
	}
	if want := []string{"metrics", "tracing"}; !equalStrings(features, want) {
		t.Fatalf("features = %v, want %v", features, want)
	}
	if got := cfg.Scope(); got != "default" {
		t.Fatalf("scope = %q", got)
	}
	if !cfg.IsSecret("billing.api_key") {
		t.Fatal("billing.api_key should be reported as resolved from a reference")
	}
	if ref, ok := cfg.Ref("billing.api_key"); !ok || ref != "secret://kv/billing/api_key" {
		t.Fatalf("Ref = %q, %v", ref, ok)
	}
}

func TestLoadStagingScope(t *testing.T) {
	cfg := mustLoad(t, loadOptions(t, "../../testdata/base.json", "", "staging"))

	if got, _ := cfg.Get("billing.api_key"); got != "kv-staging-billing-key" {
		t.Fatalf("billing.api_key = %q", got)
	}
	if got, _ := cfg.Get("billing.publishable_key"); got != "pk-live-123" {
		t.Fatalf("billing.publishable_key = %q", got)
	}
}

func TestLoadReportsResolutionFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unresolvable.json")
	body := `{"db.password": "secret://kv/billing/absent"}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	opts := loadOptions(t, path, "", "default")
	cfg, err := Load(opts)
	if err == nil {
		t.Fatalf("expected an error, got config %v", cfg)
	}
	if !strings.Contains(err.Error(), "billing/absent") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadRequiresBasePath(t *testing.T) {
	if _, err := Load(Options{Resolver: resolverFor(t)}); err == nil {
		t.Fatal("expected an error when the base path is empty")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
