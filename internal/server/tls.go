// Package server builds the server side of the mutual-TLS handshake.
package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
)

// Config carries the material a peer-facing server needs.
type Config struct {
	// Certificate is the server's own certificate chain.
	Certificate tls.Certificate
	// ClientCAs holds the anchors a peer certificate is verified against.
	ClientCAs *x509.CertPool
}

// NewServerTLSConfig builds the server-side TLS configuration.
//
// Peers are required to present a client certificate. The certificate is
// checked against the configured client CAs during the handshake.
func NewServerTLSConfig(cfg Config) (*tls.Config, error) {
	if len(cfg.Certificate.Certificate) == 0 {
		return nil, fmt.Errorf("server: no server certificate configured")
	}
	if cfg.ClientCAs == nil {
		return nil, fmt.Errorf("server: no client CAs configured")
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cfg.Certificate},
		ClientAuth:   tls.RequireAnyClientCert,
		ClientCAs:    cfg.ClientCAs,
	}, nil
}
