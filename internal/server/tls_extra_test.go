package server

import (
	"crypto/tls"
	"errors"
	"testing"

	"peerloom/internal/certtest"
)

func TestHandshakeRejectsSelfSignedClientCertificate(t *testing.T) {
	ca := certtest.NewCA(t, "dev-root")
	srv := ca.Issue(t, certtest.Options{
		CommonName: "localhost",
		DNSNames:   []string{"localhost"},
		Server:     true,
	})
	selfSigned := certtest.SelfSigned(t, certtest.Options{
		CommonName: "payments",
		URIs:       []string{"spiffe://corp/payments/i-1"},
		Client:     true,
	})

	cfg, err := NewServerTLSConfig(Config{Certificate: srv.TLS, ClientCAs: ca.Pool})
	if err != nil {
		t.Fatalf("NewServerTLSConfig: %v", err)
	}
	if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Fatalf("ClientAuth = %v, want full client certificate verification", cfg.ClientAuth)
	}

	if _, err := handshake(t, cfg, selfSigned); err == nil {
		t.Fatal("handshake accepted a self-signed client certificate")
	}
}

func TestHandshakeRejectsCertificateFromUnregisteredCA(t *testing.T) {
	registered := certtest.NewCA(t, "registered-root")
	rogue := certtest.NewCA(t, "rogue-root")
	srv := registered.Issue(t, certtest.Options{
		CommonName: "localhost",
		DNSNames:   []string{"localhost"},
		Server:     true,
	})
	rogueClient := rogue.Issue(t, certtest.Options{
		CommonName: "i-rogue",
		URIs:       []string{"spiffe://corp/payments/i-rogue"},
		Client:     true,
	})

	cfg, err := NewServerTLSConfig(Config{Certificate: srv.TLS, ClientCAs: registered.Pool})
	if err != nil {
		t.Fatalf("NewServerTLSConfig: %v", err)
	}

	if _, err := handshake(t, cfg, rogueClient); err == nil {
		t.Fatal("handshake accepted a certificate issued by an unregistered CA")
	}
}

type staticVerifier struct{ err error }

func (v staticVerifier) VerifyPeer([][]byte) error { return v.err }

func TestHandshakeUsesCustomPeerVerifier(t *testing.T) {
	ca := certtest.NewCA(t, "dev-root")
	srv := ca.Issue(t, certtest.Options{
		CommonName: "localhost",
		DNSNames:   []string{"localhost"},
		Server:     true,
	})
	client := ca.Issue(t, certtest.Options{
		CommonName: "payments",
		URIs:       []string{"spiffe://corp/payments/i-1"},
		Client:     true,
	})

	wantErr := errors.New("epoch verification failed")
	cfg, err := NewServerTLSConfig(Config{
		Certificate:  srv.TLS,
		ClientCAs:    ca.Pool,
		PeerVerifier: staticVerifier{err: wantErr},
	})
	if err != nil {
		t.Fatalf("NewServerTLSConfig: %v", err)
	}
	if cfg.VerifyPeerCertificate == nil {
		t.Fatal("custom PeerVerifier was not wired into the TLS config")
	}

	if _, err := handshake(t, cfg, client); !errors.Is(err, wantErr) {
		t.Fatalf("handshake err = %v, want %v", err, wantErr)
	}

	okCfg, err := NewServerTLSConfig(Config{
		Certificate:  srv.TLS,
		ClientCAs:    ca.Pool,
		PeerVerifier: staticVerifier{},
	})
	if err != nil {
		t.Fatalf("NewServerTLSConfig: %v", err)
	}
	if _, err := handshake(t, okCfg, client); err != nil {
		t.Fatalf("handshake with accepting verifier: %v", err)
	}
}

func TestNewServerTLSConfigWithoutTrustMaterial(t *testing.T) {
	ca := certtest.NewCA(t, "dev-root")
	srv := ca.Issue(t, certtest.Options{CommonName: "localhost", Server: true})

	if _, err := NewServerTLSConfig(Config{Certificate: srv.TLS}); err == nil {
		t.Fatal("NewServerTLSConfig accepted a config without any client trust material")
	}
}
