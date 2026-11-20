package tlspool

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"path/filepath"
	"testing"
	"time"
)

func selfSignedServerCertificate(t *testing.T, name string) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{name},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create self-signed certificate: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func TestBuildClientTLSRejectsUntrustedServer(t *testing.T) {
	t.Parallel()

	goodDir := filepath.Join("..", "..", "testdata", "good")
	rogueDir := filepath.Join("..", "..", "testdata", "rogue")
	clientCfg := BuildClientTLS(ClientConfig{
		RootCAs:    loadPool(t, filepath.Join(goodDir, "ca.pem")),
		ServerName: "rogue.example.com",
	})
	serverCfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{loadKeyPair(t, filepath.Join(rogueDir, "leaf.pem"), filepath.Join(rogueDir, "leaf-key.pem"))},
	}
	if err := pairHandshake(serverCfg, clientCfg); err == nil {
		t.Fatal("handshake with a server from an untrusted authority succeeded")
	}
}

func TestBuildClientTLSRejectsSelfSignedServer(t *testing.T) {
	t.Parallel()

	goodDir := filepath.Join("..", "..", "testdata", "good")
	clientCfg := BuildClientTLS(ClientConfig{
		RootCAs:    loadPool(t, filepath.Join(goodDir, "ca.pem")),
		ServerName: "localhost",
	})
	serverCfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{selfSignedServerCertificate(t, "localhost")},
	}
	if err := pairHandshake(serverCfg, clientCfg); err == nil {
		t.Fatal("handshake with a self-signed server succeeded without opting in")
	}
}

func TestBuildClientTLSAllowSelfSignedOptIn(t *testing.T) {
	t.Parallel()

	goodDir := filepath.Join("..", "..", "testdata", "good")
	clientCfg := BuildClientTLS(ClientConfig{
		RootCAs:         loadPool(t, filepath.Join(goodDir, "ca.pem")),
		ServerName:      "localhost",
		AllowSelfSigned: true,
	})
	serverCfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{selfSignedServerCertificate(t, "localhost")},
	}
	if err := pairHandshake(serverCfg, clientCfg); err != nil {
		t.Fatalf("handshake with AllowSelfSigned failed: %v", err)
	}
}
