package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePolicyDocument(t *testing.T) {
	data := []byte(`{
		"version": "1",
		"rules": [
			{"name": "admin", "roles": ["admin"], "resource": "/**", "actions": ["*"], "effect": "allow"}
		]
	}`)
	p, err := ParsePolicy(data)
	if err != nil {
		t.Fatalf("ParsePolicy: %v", err)
	}
	if p.Version != "1" {
		t.Fatalf("version = %q, want %q", p.Version, "1")
	}
	if len(p.Rules) != 1 || p.Rules[0].Effect != Allow {
		t.Fatalf("unexpected rules: %+v", p.Rules)
	}
}

func TestParsePolicyRejectsInvalidEffect(t *testing.T) {
	data := []byte(`{"rules": [{"roles": ["a"], "resource": "/x", "effect": "maybe"}]}`)
	if _, err := ParsePolicy(data); err == nil {
		t.Fatal("expected an error for an unknown effect")
	}
}

func TestLoadFileDemoPolicy(t *testing.T) {
	path := filepath.Join("..", "..", "policies", "demo.json")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("demo policy not available: %v", err)
	}
	p, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if len(p.Rules) == 0 {
		t.Fatal("expected the demo policy to carry rules")
	}
}
