package server

import (
	"crypto/tls"
	"net"
	"strings"
	"testing"

	"peerloom/internal/certtest"
)

func TestNewServerTLSConfigRequiresConfig(t *testing.T) {
	if _, err := NewServerTLSConfig(Config{}); err == nil {
		t.Fatal("NewServerTLSConfig accepted an empty config")
	}
}

func TestNewServerTLSConfigAsksForClientCertificate(t *testing.T) {
	ca := certtest.NewCA(t, "dev-root")
	srv := ca.Issue(t, certtest.Options{
		CommonName: "localhost",
		DNSNames:   []string{"localhost"},
		Server:     true,
	})

	cfg, err := NewServerTLSConfig(Config{Certificate: srv.TLS, ClientCAs: ca.Pool})
	if err != nil {
		t.Fatalf("NewServerTLSConfig: %v", err)
	}
	if cfg.ClientAuth == tls.NoClientCert {
		t.Fatal("server config does not ask for a client certificate")
	}
	if cfg.ClientCAs == nil {
		t.Fatal("server config has no client CA pool")
	}
	if cfg.MinVersion < tls.VersionTLS12 {
		t.Errorf("MinVersion = %x, want at least TLS 1.2", cfg.MinVersion)
	}
}

func TestHandshakeAcceptsRegisteredClient(t *testing.T) {
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

	cfg, err := NewServerTLSConfig(Config{Certificate: srv.TLS, ClientCAs: ca.Pool})
	if err != nil {
		t.Fatalf("NewServerTLSConfig: %v", err)
	}

	cs, err := handshake(t, cfg, client)
	if err != nil {
		t.Fatalf("handshake: %v", err)
	}
	if len(cs.PeerCertificates) == 0 {
		t.Fatal("handshake completed without a peer certificate")
	}
	if !strings.Contains(cs.PeerCertificates[0].Subject.CommonName, "payments") {
		t.Errorf("peer CN = %q, want payments", cs.PeerCertificates[0].Subject.CommonName)
	}
}

// handshake runs a full TLS handshake over a loopback listener and returns the
// server's view of the connection.
func handshake(t *testing.T, cfg *tls.Config, client *certtest.Cert) (*tls.ConnectionState, error) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	type result struct {
		state *tls.ConnectionState
		err   error
	}
	done := make(chan result, 1)
	go func() {
		raw, err := ln.Accept()
		if err != nil {
			done <- result{err: err}
			return
		}
		defer raw.Close()
		conn := tls.Server(raw, cfg)
		if err := conn.Handshake(); err != nil {
			done <- result{err: err}
			return
		}
		cs := conn.ConnectionState()
		done <- result{state: &cs}
	}()

	raw, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer raw.Close()

	cli := tls.Client(raw, &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{client.TLS},
	})
	defer cli.Close()
	if err := cli.Handshake(); err != nil {
		<-done
		return nil, err
	}

	res := <-done
	return res.state, res.err
}
