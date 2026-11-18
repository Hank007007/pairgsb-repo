package identity

import (
	"crypto/x509"
	"net/url"
	"testing"
)

func newCert(t *testing.T, uris ...*url.URL) *x509.Certificate {
	t.Helper()
	return &x509.Certificate{URIs: uris}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

func TestExtractIDReadsServiceAndInstance(t *testing.T) {
	cert := newCert(t, mustURL(t, "spiffe://corp/payments/i-7f3a"))

	id, err := ExtractID(cert, "corp")
	if err != nil {
		t.Fatalf("ExtractID: %v", err)
	}
	if id.Service != "payments" {
		t.Errorf("Service = %q, want %q", id.Service, "payments")
	}
	if id.Instance != "i-7f3a" {
		t.Errorf("Instance = %q, want %q", id.Instance, "i-7f3a")
	}
	if id.TrustDomain != "corp" {
		t.Errorf("TrustDomain = %q, want %q", id.TrustDomain, "corp")
	}
}

func TestExtractIDRequiresCertificate(t *testing.T) {
	if _, err := ExtractID(nil, "corp"); err != ErrNoCertificate {
		t.Fatalf("err = %v, want ErrNoCertificate", err)
	}
}
