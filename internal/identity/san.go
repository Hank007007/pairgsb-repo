package identity

import (
	"crypto/x509"
	"net/url"
)

// ExtractID derives the peer's service identity from its certificate.
//
// The identity comes exclusively from the certificate's URI SANs. A cert
// without a usable URI SAN is rejected; DNS names, the common name and other
// fields are never used to infer an identity. Every URI SAN must parse as a
// SPIFFE identity under trustDomain and resolve to the same service, otherwise
// the whole certificate is rejected rather than picking one SAN.
func ExtractID(cert *x509.Certificate, trustDomain string) (Identity, error) {
	if cert == nil {
		return Identity{}, ErrNoCertificate
	}
	if len(cert.URIs) == 0 {
		return Identity{}, ErrNoIdentity
	}

	var id Identity
	found := false
	for _, u := range cert.URIs {
		next, err := parseSAN(u, trustDomain)
		if err != nil {
			return Identity{}, err
		}
		if !found {
			id = next
			found = true
			continue
		}
		if next.TrustDomain != id.TrustDomain || next.Service != id.Service {
			return Identity{}, ErrNoIdentity
		}
	}

	if !found {
		return Identity{}, ErrNoIdentity
	}
	return id, nil
}

// parseSAN validates one URI SAN against the SPIFFE shape
// spiffe://<trust-domain>/<service>[/<instance>] and returns the identity it
// carries.
func parseSAN(u *url.URL, trustDomain string) (Identity, error) {
	if u == nil {
		return Identity{}, ErrNoIdentity
	}
	if u.Scheme != "spiffe" {
		return Identity{}, ErrBadURI
	}
	// Host comparison follows URI authority rules; a different port is a
	// different authority and therefore a different trust domain.
	if u.Host != trustDomain || u.Host == "" {
		return Identity{}, ErrNoIdentity
	}
	if u.Opaque != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return Identity{}, ErrBadURI
	}

	path := u.EscapedPath()
	if len(path) < 2 || path[0] != '/' || path[1] == '/' {
		return Identity{}, ErrBadURI
	}
	rest := path[1:]
	if rest[len(rest)-1] == '/' {
		return Identity{}, ErrBadURI
	}

	var segments []string
	start := 0
	for i := 0; i <= len(rest); i++ {
		if i != len(rest) && rest[i] != '/' {
			continue
		}
		seg, err := validSegment(rest[start:i])
		if err != nil {
			return Identity{}, err
		}
		segments = append(segments, seg)
		start = i + 1
	}
	if len(segments) != 1 && len(segments) != 2 {
		return Identity{}, ErrBadURI
	}

	id := Identity{TrustDomain: u.Host, Service: segments[0]}
	if len(segments) == 2 {
		id.Instance = segments[1]
	}
	return id, nil
}

// validSegment rejects empty path segments, dot segments and percent-encoded
// slashes that would smuggle extra separators past the split above.
func validSegment(raw string) (string, error) {
	if raw == "" || raw == "." || raw == ".." {
		return "", ErrBadURI
	}
	seg, err := url.PathUnescape(raw)
	if err != nil {
		return "", ErrBadURI
	}
	if seg == "" || seg == "." || seg == ".." {
		return "", ErrBadURI
	}
	for _, r := range seg {
		if r == '/' {
			return "", ErrBadURI
		}
	}
	return seg, nil
}
