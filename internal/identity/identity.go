// Package identity models the calling peer.
//
// A peer is identified by a SPIFFE-style URI SAN of the form
//
//	spiffe://<trust-domain>/<service>/<instance>
//
// Only the service segment participates in authorization; the instance is
// carried for audit purposes.
package identity

import (
	"errors"
	"fmt"
	"net/url"
)

// Errors returned while turning a certificate into an Identity.
var (
	ErrNoCertificate = errors.New("identity: peer did not present a certificate")
	ErrNoIdentity    = errors.New("identity: certificate carries no usable identity")
	ErrBadURI        = errors.New("identity: malformed spiffe URI")
)

// Identity is the service identity of a peer.
type Identity struct {
	// TrustDomain is the authority the identity belongs to.
	TrustDomain string
	// Service is the service segment of the URI SAN.
	Service string
	// Instance names a particular running instance of Service.
	Instance string
}

// IsZero reports whether the identity carries no service at all.
func (i Identity) IsZero() bool {
	return i.Service == ""
}

// String renders the identity in the same shape as the URI SAN it came from.
func (i Identity) String() string {
	if i.IsZero() {
		return ""
	}
	if i.Instance == "" {
		return fmt.Sprintf("spiffe://%s/%s", i.TrustDomain, i.Service)
	}
	return fmt.Sprintf("spiffe://%s/%s/%s", i.TrustDomain, i.Service, i.Instance)
}

// URI returns the identity as a *url.URL. It is used when constructing
// certificates and audit records.
func (i Identity) URI() *url.URL {
	if i.IsZero() {
		return nil
	}
	p := "/" + i.Service
	if i.Instance != "" {
		p += "/" + i.Instance
	}
	return &url.URL{Scheme: "spiffe", Host: i.TrustDomain, Path: p}
}
