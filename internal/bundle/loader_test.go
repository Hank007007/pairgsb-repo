package bundle

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"peerloom/internal/certtest"
)

// writeEpoch materialises one epoch directory. roots and inter may be nil.
func writeEpoch(t *testing.T, dir, id string, roots, inter []byte) {
	t.Helper()
	for _, sub := range []string{"anchors", "intermediates"} {
		if err := os.MkdirAll(filepath.Join(dir, id, sub), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if roots != nil {
		writeFile(t, filepath.Join(dir, id, "anchors", "root.pem"), roots)
	}
	if inter != nil {
		writeFile(t, filepath.Join(dir, id, "intermediates", "inter.pem"), inter)
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeManifest(t *testing.T, dir, body string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "epochs.json"), []byte(body))
}

func TestLoadDirReadsSingleEpoch(t *testing.T) {
	ca := certtest.NewCA(t, "dev-root")
	dir := t.TempDir()
	writeEpoch(t, dir, "e0001", ca.CAPEM(), nil)
	writeManifest(t, dir, `{"current":"e0001","active":["e0001"],"retired":[]}`)

	b, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if b.Current != "e0001" {
		t.Errorf("Current = %q, want e0001", b.Current)
	}
	ep, ok := b.Epoch("e0001")
	if !ok {
		t.Fatal("epoch e0001 was not loaded")
	}
	if ep.Roots == nil {
		t.Fatal("epoch e0001 has no roots")
	}
	if got := len(b.Verifiable()); got != 1 {
		t.Errorf("Verifiable() returned %d epochs, want 1", got)
	}
}

func TestVerifyAcceptsChainThroughIntermediate(t *testing.T) {
	root := certtest.NewCA(t, "dev-root")

	// Issue the intermediate as a CA signed by the root.
	inter := root.Issue(t, certtest.Options{CommonName: "dev-inter", IsCA: true})

	leaf := inter.AsCA(t).Issue(t, certtest.Options{
		CommonName: "payments",
		URIs:       []string{"spiffe://corp/payments/i-1"},
		Client:     true,
	})

	dir := t.TempDir()
	writeEpoch(t, dir, "e0001", root.CAPEM(), inter.PEM())
	writeManifest(t, dir, `{"current":"e0001","active":["e0001"],"retired":[]}`)

	b, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	chains, err := b.Verify([][]byte{leaf.X509.Raw}, time.Now())
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(chains) == 0 || len(chains[0]) < 2 {
		t.Fatalf("Verify returned %d chains; want one chain of length >= 2", len(chains))
	}
}

func TestRetiredEpochIsNotVerifiable(t *testing.T) {
	root := certtest.NewCA(t, "dev-root")

	dir := t.TempDir()
	writeEpoch(t, dir, "e0001", root.CAPEM(), nil)
	writeEpoch(t, dir, "e0002", root.CAPEM(), nil)
	writeManifest(t, dir, `{"current":"e0002","active":["e0001","e0002"],"retired":["e0001"]}`)

	b, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	for _, ep := range b.Verifiable() {
		if ep.ID == "e0001" {
			t.Fatal("retired epoch e0001 is still verifiable")
		}
	}
}
