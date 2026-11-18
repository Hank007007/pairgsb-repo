package policy

import (
	"os"
	"path/filepath"
	"testing"
)

const repoPolicies = "../../policies"

func TestLoadReadsSamplePolicies(t *testing.T) {
	set, err := Load(repoPolicies)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if set.Len() == 0 {
		t.Fatal("Load returned an empty rule set")
	}
	if set.Version < 3 {
		t.Errorf("Version = %d, want at least 3", set.Version)
	}
}

func TestCheckPermitsExactMatch(t *testing.T) {
	set, err := Load(repoPolicies)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if err := set.Check("payments", "/ledger.v1.Ledger/Debit"); err != nil {
		t.Fatalf("Check(payments, Debit) = %v, want nil", err)
	}
	if err := set.Check("auditor", "/ledger.v1.Audit/Events"); err != nil {
		t.Fatalf("Check(auditor, Events) = %v, want nil", err)
	}
}

func TestCheckDeniesUnlistedMethod(t *testing.T) {
	set, err := Load(repoPolicies)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if err := set.Check("payments", "/ledger.v1.Audit/Events"); err == nil {
		t.Fatal("Check(payments, Events) = nil, want an error")
	}
}

func TestAllowMatchesWildcardMethodSegment(t *testing.T) {
	rule := Rule{ID: "r", From: "reporting", To: "/ledger.v1.Ledger/*", Effect: EffectPermit}

	if !Allow("reporting", "/ledger.v1.Ledger/Balance", rule) {
		t.Error("Allow(reporting, Balance) = false, want true")
	}
}

func TestLoadRejectsMalformedRule(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "trust.json"), `{"trust_domain":"corp"}`)
	writeFile(t, filepath.Join(dir, "bad.json"), `{
		"name": "bad",
		"version": 1,
		"rules": [{"id":"x","from":"payments","to":"/svc/M","effect":"maybe"}]
	}`)

	if _, err := Load(dir); err == nil {
		t.Fatal("Load accepted a rule with an unknown effect")
	}
}

func TestLoadTrustReadsDomain(t *testing.T) {
	trust, err := LoadTrust(filepath.Join(repoPolicies, "trust.json"))
	if err != nil {
		t.Fatalf("LoadTrust: %v", err)
	}
	if trust.TrustDomain != "corp" {
		t.Errorf("TrustDomain = %q, want %q", trust.TrustDomain, "corp")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
