package store

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "secrets.json", `{
	  "secrets": [
	    {"path": "kv/app/payments/stripe", "value": "sk_live_x", "backend": "payments-core"},
	    {"path": "kv/app/ops/deploy-key", "value": "ops-key", "backend": "ops-deploy"}
	  ]
	}`)

	kv := New()
	if err := kv.LoadFile(p); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if kv.Len() != 2 {
		t.Fatalf("Len = %d, want 2", kv.Len())
	}
	s, ok := kv.Get("kv/app/payments/stripe")
	if !ok {
		t.Fatal("expected secret to be present")
	}
	if s.Value != "sk_live_x" || s.Backend != "payments-core" {
		t.Fatalf("unexpected secret: %+v", s)
	}
	if _, ok := kv.Get("kv/app/missing"); ok {
		t.Fatal("expected missing path to be absent")
	}
}

func TestLoadFileRejectsDuplicatePaths(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "dup.json", `{
	  "secrets": [
	    {"path": "kv/app/x", "value": "a", "backend": "b"},
	    {"path": "kv/app/x", "value": "c", "backend": "d"}
	  ]
	}`)
	if err := New().LoadFile(p); err == nil {
		t.Fatal("expected duplicate paths to be rejected")
	}
}

func TestLoadFileRejectsIncompleteRecords(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"no-value.json":   `{"secrets":[{"path":"kv/app/x","backend":"b"}]}`,
		"no-backend.json": `{"secrets":[{"path":"kv/app/x","value":"a"}]}`,
		"no-path.json":    `{"secrets":[{"value":"a","backend":"b"}]}`,
	} {
		p := writeFile(t, dir, name, body)
		if err := New().LoadFile(p); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestLoadTestdataSecrets(t *testing.T) {
	kv := New()
	if err := kv.LoadFile("../../testdata/secrets.json"); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if kv.Len() != 5 {
		t.Fatalf("Len = %d, want 5", kv.Len())
	}
}

func TestPathsSorted(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "s.json", `{"secrets":[
	  {"path":"kv/b","value":"1","backend":"x"},
	  {"path":"kv/a","value":"2","backend":"x"}
	]}`)
	kv := New()
	if err := kv.LoadFile(p); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	paths := kv.Paths()
	if len(paths) != 2 || paths[0] != "kv/a" || paths[1] != "kv/b" {
		t.Fatalf("Paths = %v", paths)
	}
}
