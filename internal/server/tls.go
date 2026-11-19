// Package server builds the server side of the mutual-TLS handshake.
package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
)

// PeerVerifier performs the full certificate-chain check for a peer's
// certificate during the TLS handshake. rawChains is the DER-encoded chain
// the peer presented, leaf first. A non-nil error aborts the handshake with a
// TLS alert before any RPC layer is reached.
type PeerVerifier interface {
	VerifyPeer(rawChains [][]byte) error
}

// Config carries the material a peer-facing server needs.
type Config struct {
	// Certificate is the server's own certificate chain.
	Certificate tls.Certificate
	// ClientCAs holds the anchors a peer certificate is verified against.
	ClientCAs *x509.CertPool
	// PeerVerifier optionally replaces the built-in chain verification with a
	// custom check, e.g. verification scoped to a trust-bundle epoch. When it
	// is set, the peer certificate is still mandatory, but ClientCAs is used
	// only to request a certificate; the verifier owns the trust decision.
	PeerVerifier PeerVerifier
}

// NewServerTLSConfig builds the server-side TLS configuration.
//
// Peers are required to present a client certificate. The certificate is
// checked against the configured client CAs during the handshake.
func NewServerTLSConfig(cfg Config) (*tls.Config, error) {
	if len(cfg.Certificate.Certificate) == 0 {
		return nil, fmt.Errorf("server: no server certificate configured")
	}
	if cfg.ClientCAs == nil && cfg.PeerVerifier == nil {
		return nil, fmt.Errorf("server: no client CAs configured")
	}

	tlsCfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cfg.Certificate},
		ClientCAs:    cfg.ClientCAs,
	}

	if cfg.PeerVerifier != nil {
		// A peer certificate is still mandatory; the verifier performs the
		// complete chain check so a self-signed or unregistered certificate
		// fails inside the handshake.
		tlsCfg.ClientAuth = tls.RequireAnyClientCert
		verifier := cfg.PeerVerifier
		tlsCfg.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return fmt.Errorf("server: peer presented no certificate")
			}
			return verifier.VerifyPeer(rawCerts)
		}
	} else {
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsCfg, nil
}
