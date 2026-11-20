package loader

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func loadPoolForRegression(t *testing.T, path string) *x509.CertPool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureRoot, path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		t.Fatalf("%s contained no certificates", path)
	}
	return pool
}

func regressionNow() time.Time {
	return time.Date(2030, 6, 1, 12, 0, 0, 0, time.UTC)
}

func TestLoadPairRejectsIssuerOutsideTrustPool(t *testing.T) {
	t.Parallel()

	_, err := LoadPair(Config{
		CertFile:           fixture("rogue/leaf.pem"),
		KeyFile:            fixture("rogue/leaf-key.pem"),
		RootCAs:            loadPoolForRegression(t, "good/ca.pem"),
		ExpectedIdentities: []string{"rogue.example.com"},
		Now:                regressionNow,
	})
	if err == nil {
		t.Fatal("expected loading a certificate from an untrusted issuer to fail")
	}
	if !strings.Contains(err.Error(), "not trusted") {
		t.Fatalf("error = %v, want a trust-pool failure", err)
	}
}

func TestLoadPairRejectsWrongIdentity(t *testing.T) {
	t.Parallel()

	_, err := LoadPair(Config{
		CertFile:           fixture("misnamed/leaf.pem"),
		KeyFile:            fixture("misnamed/leaf-key.pem"),
		RootCAs:            loadPoolForRegression(t, "misnamed/ca.pem"),
		ExpectedIdentities: []string{"service.internal"},
		Now:                regressionNow,
	})
	if err == nil {
		t.Fatal("expected loading a certificate for a different identity to fail")
	}
	if !strings.Contains(err.Error(), "not valid for any expected identity") {
		t.Fatalf("error = %v, want an identity failure", err)
	}
}

func TestLoadPairAcceptsTrustedNamedCertificate(t *testing.T) {
	t.Parallel()

	pair, err := LoadPair(Config{
		CertFile:           fixture("good/leaf.pem"),
		KeyFile:            fixture("good/leaf-key.pem"),
		RootCAs:            loadPoolForRegression(t, "good/ca.pem"),
		ExpectedIdentities: []string{"service.internal"},
		Now:                regressionNow,
	})
	if err != nil {
		t.Fatalf("LoadPair: %v", err)
	}
	if pair.Leaf == nil {
		t.Fatal("LoadPair returned a pair without a parsed leaf")
	}
}
