package bundle

import (
	"testing"

	"peerloom/internal/certtest"
)

func TestVerifyPeerAcceptsChainFromActiveEpoch(t *testing.T) {
	root := certtest.NewCA(t, "dev-root")
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
	if err := b.VerifyPeer([][]byte{leaf.X509.Raw}); err != nil {
		t.Fatalf("VerifyPeer: %v", err)
	}
}

func TestVerifyPeerRejectsUnregisteredCA(t *testing.T) {
	registered := certtest.NewCA(t, "registered-root")
	rogue := certtest.NewCA(t, "rogue-root")
	rogueLeaf := rogue.Issue(t, certtest.Options{
		CommonName: "i-rogue",
		URIs:       []string{"spiffe://corp/payments/i-rogue"},
		Client:     true,
	})

	dir := t.TempDir()
	writeEpoch(t, dir, "e0001", registered.CAPEM(), nil)
	writeManifest(t, dir, `{"current":"e0001","active":["e0001"],"retired":[]}`)

	b, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if err := b.VerifyPeer([][]byte{rogueLeaf.X509.Raw}); err == nil {
		t.Fatal("VerifyPeer accepted a certificate from an unregistered CA")
	}
}

func TestVerifyPeerRejectsRetiredEpoch(t *testing.T) {
	oldRoot := certtest.NewCA(t, "old-root")
	newRoot := certtest.NewCA(t, "new-root")
	oldLeaf := oldRoot.Issue(t, certtest.Options{
		CommonName: "payments",
		URIs:       []string{"spiffe://corp/payments/i-old"},
		Client:     true,
	})

	dir := t.TempDir()
	writeEpoch(t, dir, "e0001", oldRoot.CAPEM(), nil)
	writeEpoch(t, dir, "e0002", newRoot.CAPEM(), nil)
	writeManifest(t, dir, `{"current":"e0002","active":["e0001","e0002"],"retired":["e0001"]}`)

	b, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if err := b.VerifyPeer([][]byte{oldLeaf.X509.Raw}); err == nil {
		t.Fatal("VerifyPeer accepted a certificate from a retired epoch")
	}
}

func TestVerifyPeerRejectsCertificateWithoutClientEKU(t *testing.T) {
	root := certtest.NewCA(t, "dev-root")
	serverOnly := root.Issue(t, certtest.Options{
		CommonName: "localhost",
		DNSNames:   []string{"localhost"},
		Server:     true,
	})

	dir := t.TempDir()
	writeEpoch(t, dir, "e0001", root.CAPEM(), nil)
	writeManifest(t, dir, `{"current":"e0001","active":["e0001"],"retired":[]}`)

	b, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if err := b.VerifyPeer([][]byte{serverOnly.X509.Raw}); err == nil {
		t.Fatal("VerifyPeer accepted a certificate without client authentication EKU")
	}
}

func TestVerifyPeerRejectsEmptyChain(t *testing.T) {
	root := certtest.NewCA(t, "dev-root")
	dir := t.TempDir()
	writeEpoch(t, dir, "e0001", root.CAPEM(), nil)
	writeManifest(t, dir, `{"current":"e0001","active":["e0001"],"retired":[]}`)

	b, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if err := b.VerifyPeer(nil); err == nil {
		t.Fatal("VerifyPeer accepted an empty chain")
	}
}
