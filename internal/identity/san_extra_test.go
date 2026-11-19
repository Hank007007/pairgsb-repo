package identity

import (
	"crypto/x509"
	"errors"
	"net/url"
	"testing"
)

func certWithURIs(t *testing.T, raw ...string) *x509.Certificate {
	t.Helper()
	c := &x509.Certificate{}
	for _, s := range raw {
		u, err := url.Parse(s)
		if err != nil {
			t.Fatalf("parse %q: %v", s, err)
		}
		c.URIs = append(c.URIs, u)
	}
	return c
}

func TestExtractIDRejectsWrongTrustDomain(t *testing.T) {
	cert := certWithURIs(t, "spiffe://evil.example/payments/i-1")

	if _, err := ExtractID(cert, "corp"); !errors.Is(err, ErrNoIdentity) {
		t.Fatalf("err = %v, want ErrNoIdentity", err)
	}
}

func TestExtractIDRejectsNonSpiffeScheme(t *testing.T) {
	cert := certWithURIs(t, "https://corp/payments/i-1")

	if _, err := ExtractID(cert, "corp"); !errors.Is(err, ErrBadURI) {
		t.Fatalf("err = %v, want ErrBadURI", err)
	}
}

func TestExtractIDRejectsCertificateWithoutURISAN(t *testing.T) {
	cert := certWithURIs(t)
	cert.DNSNames = []string{"payments"}

	if _, err := ExtractID(cert, "corp"); !errors.Is(err, ErrNoIdentity) {
		t.Fatalf("err = %v, want ErrNoIdentity (DNS SAN must not be used)", err)
	}
}

func TestExtractIDRejectsConflictingURISANs(t *testing.T) {
	cert := certWithURIs(t,
		"spiffe://telemetry.internal/collector",
		"spiffe://corp/payments/i-1",
	)

	if _, err := ExtractID(cert, "corp"); err == nil {
		t.Fatal("ExtractID accepted a certificate with conflicting URI SANs")
	}
}

func TestExtractIDAcceptsConsistentURISANs(t *testing.T) {
	cert := certWithURIs(t,
		"spiffe://corp/payments/i-1",
		"spiffe://corp/payments/i-2",
	)

	id, err := ExtractID(cert, "corp")
	if err != nil {
		t.Fatalf("ExtractID: %v", err)
	}
	if id.Service != "payments" {
		t.Errorf("Service = %q, want payments", id.Service)
	}
}

func TestExtractIDValidatesPathShape(t *testing.T) {
	bad := []string{
		"spiffe://corp/",
		"spiffe://corp//i-1",
		"spiffe://corp/payments/",
		"spiffe://corp/payments/i-1/extra",
		"spiffe://corp/./i-1",
		"spiffe://corp/payments/..",
	}
	for _, raw := range bad {
		cert := certWithURIs(t, raw)
		if _, err := ExtractID(cert, "corp"); err == nil {
			t.Errorf("ExtractID accepted malformed URI %q", raw)
		}
	}
}

func TestExtractIDAcceptsSingleSegmentIdentity(t *testing.T) {
	cert := certWithURIs(t, "spiffe://corp/payments")

	id, err := ExtractID(cert, "corp")
	if err != nil {
		t.Fatalf("ExtractID: %v", err)
	}
	if id.Service != "payments" || id.Instance != "" {
		t.Errorf("identity = %+v, want service payments without instance", id)
	}
}
