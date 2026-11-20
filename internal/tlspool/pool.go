// Package tlspool builds client and server TLS configurations that share one
// certificate authority pool.
package tlspool

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
)

// ClientConfig describes a client-side TLS configuration.
type ClientConfig struct {
	// RootCAs is the pool used to verify the peer's certificate chain.
	RootCAs *x509.CertPool
	// ServerName is the name expected in the peer certificate. When empty,
	// crypto/tls uses the dial address.
	ServerName string
	// AllowSelfSigned permits peers that do not chain to a known authority.
	// It is intended for local development only.
	AllowSelfSigned bool
	// StrictVerify is retained for older call sites and has no effect.
	// Verification is always enabled unless AllowSelfSigned is set.
	StrictVerify bool
}

// BuildClientTLS returns a client configuration that verifies the peer against
// cfg.RootCAs.
func BuildClientTLS(cfg ClientConfig) *tls.Config {
	out := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    cfg.RootCAs,
		ServerName: cfg.ServerName,
	}
	if cfg.AllowSelfSigned {
		out.InsecureSkipVerify = true
	}
	return out
}

// ServerConfig describes a server-side TLS configuration.
type ServerConfig struct {
	// Certificates is the server's own certificate chain.
	Certificates []tls.Certificate
	// ClientCAs is the pool used to verify client certificates.
	ClientCAs *x509.CertPool
	// PeerIdentities lists acceptable DNS names for connecting clients. When
	// empty, any client that chains to ClientCAs is accepted.
	PeerIdentities []string
}

// BuildServerTLS returns a server configuration that requires and verifies
// client certificates against cfg.ClientCAs.
func BuildServerTLS(cfg ServerConfig) *tls.Config {
	out := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: cfg.Certificates,
		ClientCAs:    cfg.ClientCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	if len(cfg.PeerIdentities) > 0 {
		identities := append([]string(nil), cfg.PeerIdentities...)
		out.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return errors.New("tlspool: peer presented no certificate")
			}
			peer, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return fmt.Errorf("tlspool: parse peer certificate: %w", err)
			}
			for _, name := range peer.DNSNames {
				for _, want := range identities {
					if name == want {
						return nil
					}
				}
			}
			return fmt.Errorf("tlspool: peer certificate is not valid for any configured identity")
		}
	}
	return out
}
