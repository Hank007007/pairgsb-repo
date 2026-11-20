package tlspool

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func loadPool(t *testing.T, path string) *x509.CertPool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		t.Fatalf("%s contained no certificates", path)
	}
	return pool
}

func loadKeyPair(t *testing.T, certPath, keyPath string) tls.Certificate {
	t.Helper()
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Fatalf("load key pair: %v", err)
	}
	return cert
}

// pairHandshake runs a server and client against each other over a loopback
// listener and returns the first handshake error, if any.
func pairHandshake(serverCfg, clientCfg *tls.Config) error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		raw, err := ln.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer raw.Close()
		serverErr <- tls.Server(raw, serverCfg).Handshake()
	}()

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	rawClient, err := dialer.Dial("tcp", ln.Addr().String())
	if err != nil {
		return err
	}
	clientErr := tls.Client(rawClient, clientCfg).Handshake()
	// Close the raw connection rather than the tls.Conn so that neither side
	// waits on a close-notify exchange after the handshake has finished.
	_ = rawClient.Close()
	<-serverErr
	return clientErr
}

func TestBuildClientTLSHandshakeWithTrustedServer(t *testing.T) {
	t.Parallel()

	goodDir := filepath.Join("..", "..", "testdata", "good")
	clientCfg := BuildClientTLS(ClientConfig{
		RootCAs:    loadPool(t, filepath.Join(goodDir, "ca.pem")),
		ServerName: "service.internal",
	})
	if clientCfg == nil {
		t.Fatal("BuildClientTLS returned nil")
	}
	if clientCfg.ServerName != "service.internal" {
		t.Fatalf("ServerName = %q, want service.internal", clientCfg.ServerName)
	}

	serverCfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{loadKeyPair(t, filepath.Join(goodDir, "leaf.pem"), filepath.Join(goodDir, "leaf-key.pem"))},
	}
	if err := pairHandshake(serverCfg, clientCfg); err != nil {
		t.Fatalf("handshake with a trusted server failed: %v", err)
	}
}

func TestBuildServerTLSVerifiesTrustedClient(t *testing.T) {
	t.Parallel()

	goodDir := filepath.Join("..", "..", "testdata", "good")
	serverCfg := BuildServerTLS(ServerConfig{
		Certificates:   []tls.Certificate{loadKeyPair(t, filepath.Join(goodDir, "leaf.pem"), filepath.Join(goodDir, "leaf-key.pem"))},
		ClientCAs:      loadPool(t, filepath.Join(goodDir, "ca.pem")),
		PeerIdentities: []string{"client.internal"},
	})
	if serverCfg == nil {
		t.Fatal("BuildServerTLS returned nil")
	}
	if serverCfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Fatalf("ClientAuth = %v, want RequireAndVerifyClientCert", serverCfg.ClientAuth)
	}

	// The client trusts the server; the server verifies the client.
	clientCfg := BuildClientTLS(ClientConfig{
		RootCAs:    loadPool(t, filepath.Join(goodDir, "ca.pem")),
		ServerName: "service.internal",
	})
	clientCfg.Certificates = []tls.Certificate{
		loadKeyPair(t, filepath.Join(goodDir, "client.pem"), filepath.Join(goodDir, "client-key.pem")),
	}
	if err := pairHandshake(serverCfg, clientCfg); err != nil {
		t.Fatalf("mutual handshake failed: %v", err)
	}
}

func TestBuildServerTLSWithoutIdentitiesAcceptsChainedClient(t *testing.T) {
	t.Parallel()

	goodDir := filepath.Join("..", "..", "testdata", "good")
	serverCfg := BuildServerTLS(ServerConfig{
		Certificates: []tls.Certificate{loadKeyPair(t, filepath.Join(goodDir, "leaf.pem"), filepath.Join(goodDir, "leaf-key.pem"))},
		ClientCAs:    loadPool(t, filepath.Join(goodDir, "ca.pem")),
	})
	clientCfg := BuildClientTLS(ClientConfig{
		RootCAs:    loadPool(t, filepath.Join(goodDir, "ca.pem")),
		ServerName: "service.internal",
	})
	clientCfg.Certificates = []tls.Certificate{
		loadKeyPair(t, filepath.Join(goodDir, "client.pem"), filepath.Join(goodDir, "client-key.pem")),
	}
	if err := pairHandshake(serverCfg, clientCfg); err != nil {
		t.Fatalf("handshake failed: %v", err)
	}
}
