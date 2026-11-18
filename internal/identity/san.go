package identity

import (
	"crypto/x509"
	"strings"
)

// ExtractID derives the peer's service identity from its certificate.
//
// The identity is taken from the certificate's URI SANs. When the certificate
// carries no URI SAN at all, the first DNS SAN is used instead.
func ExtractID(cert *x509.Certificate, trustDomain string) (Identity, error) {
	if cert == nil {
		return Identity{}, ErrNoCertificate
	}

	for _, u := range cert.URIs {
		parts := splitSANPath(u.Path)
		if len(parts) == 0 {
			continue
		}
		id := Identity{
			TrustDomain: u.Host,
			Service:     parts[len(parts)-1],
		}
		if len(parts) >= 2 {
			id.Service = parts[len(parts)-2]
			id.Instance = parts[len(parts)-1]
		}
		return id, nil
	}

	for _, name := range cert.DNSNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if idx := strings.IndexByte(name, '.'); idx > 0 {
			name = name[:idx]
		}
		if name == "" {
			continue
		}
		return Identity{TrustDomain: trustDomain, Service: name}, nil
	}

	return Identity{}, ErrNoIdentity
}

// splitSANPath splits a URI SAN path into its segments.
func splitSANPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}
