package rpc

import (
	"crypto/tls"
	"errors"
	"fmt"

	"peerloom/internal/identity"
)

// ErrNoPeerCertificate is returned when the connection state carries no peer
// certificate chain.
var ErrNoPeerCertificate = errors.New("rpc: connection has no peer certificate")

// PeerFromConnState derives the peer identity from a completed TLS
// connection. trustDomain is the local trust domain from policies/trust.json.
func PeerFromConnState(cs *tls.ConnectionState, trustDomain string) (identity.Identity, error) {
	if cs == nil || len(cs.PeerCertificates) == 0 {
		return identity.Identity{}, ErrNoPeerCertificate
	}
	return identity.ExtractID(cs.PeerCertificates[0], trustDomain)
}

// PeerFromConn is a convenience wrapper around PeerFromConnState.
func PeerFromConn(c *tls.Conn, trustDomain string) (identity.Identity, error) {
	if c == nil {
		return identity.Identity{}, ErrNoPeerCertificate
	}
	cs := c.ConnectionState()
	return PeerFromConnState(&cs, trustDomain)
}

// RequirePeerContext attaches the peer identity from a TLS connection to a
// fresh context, or fails when the connection carries no usable identity.
func RequirePeerContext(c *tls.Conn, trustDomain string) (identity.Identity, error) {
	id, err := PeerFromConn(c, trustDomain)
	if err != nil {
		return identity.Identity{}, err
	}
	if id.IsZero() {
		return identity.Identity{}, identity.ErrNoIdentity
	}
	if id.TrustDomain == "" {
		return identity.Identity{}, fmt.Errorf("%w: empty trust domain", identity.ErrBadURI)
	}
	return id, nil
}
