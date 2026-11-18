// Package certtest builds certificates for tests and local tools.
//
// It is intentionally small and produces ephemeral keys; nothing here is
// suitable for production use.
package certtest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"sync/atomic"
	"time"
)

// TB is the subset of testing.TB this package needs. Passing nil is allowed so
// the helpers can also be used from standalone programs; failures then panic.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
}

type silentTB struct{}

func (silentTB) Helper() {}

func (silentTB) Fatalf(format string, args ...any) {
	panic(fmt.Sprintf(format, args...))
}

func use(t TB) TB {
	if t == nil {
		return silentTB{}
	}
	t.Helper()
	return t
}

var serial atomic.Int64

func nextSerial() *big.Int {
	return big.NewInt(serial.Add(1))
}

// CA is a certificate authority usable for issuing test certificates.
type CA struct {
	Cert *x509.Certificate
	Key  *ecdsa.PrivateKey
	Pool *x509.CertPool
}

// Options describes a certificate to issue.
type Options struct {
	// CommonName is the subject common name.
	CommonName string
	// DNSNames, IPAddresses and URIs populate the respective SANs.
	DNSNames    []string
	IPAddresses []net.IP
	URIs        []string
	// Server and Client select extended key usages. When neither is set both
	// are applied.
	Server bool
	Client bool
	// NotBefore and NotAfter override the validity window.
	NotBefore time.Time
	NotAfter  time.Time
	// IsCA marks the certificate as a CA.
	IsCA bool
}

// Cert bundles the TLS and parsed forms of an issued certificate.
type Cert struct {
	TLS  tls.Certificate
	X509 *x509.Certificate
}

func newKey(t TB) *ecdsa.PrivateKey {
	t = use(t)
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("certtest: generate key: %v", err)
	}
	return k
}

func template(o Options) (*x509.Certificate, error) {
	now := time.Now()
	nb, na := o.NotBefore, o.NotAfter
	if nb.IsZero() {
		nb = now.Add(-time.Hour)
	}
	if na.IsZero() {
		na = now.Add(24 * time.Hour)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          nextSerial(),
		Subject:               pkix.Name{CommonName: o.CommonName},
		NotBefore:             nb,
		NotAfter:              na,
		BasicConstraintsValid: true,
		IsCA:                  o.IsCA,
		DNSNames:              append([]string(nil), o.DNSNames...),
		IPAddresses:           append([]net.IP(nil), o.IPAddresses...),
	}
	if o.IsCA {
		tmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	} else {
		tmpl.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	}
	for _, raw := range o.URIs {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, err
		}
		tmpl.URIs = append(tmpl.URIs, u)
	}
	server, client := o.Server, o.Client
	if !server && !client {
		server, client = true, true
	}
	if server {
		tmpl.ExtKeyUsage = append(tmpl.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
	}
	if client {
		tmpl.ExtKeyUsage = append(tmpl.ExtKeyUsage, x509.ExtKeyUsageClientAuth)
	}
	return tmpl, nil
}

// NewCA creates a self-signed CA.
func NewCA(t TB, cn string) *CA {
	t = use(t)
	tmpl, err := template(Options{CommonName: cn, IsCA: true})
	if err != nil {
		t.Fatalf("certtest: build CA template: %v", err)
	}
	key := newKey(t)
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("certtest: create CA: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("certtest: parse CA: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return &CA{Cert: cert, Key: key, Pool: pool}
}

// Issue creates a certificate signed by the CA.
func (ca *CA) Issue(t TB, o Options) *Cert {
	t = use(t)
	return issue(t, o, ca.Cert, ca.Key)
}

// AsCA views an intermediate certificate as a CA that can issue further
// certificates.
func (c *Cert) AsCA(t TB) *CA {
	t = use(t)
	key, ok := c.TLS.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatalf("certtest: certificate key is not an ECDSA key")
	}
	pool := x509.NewCertPool()
	pool.AddCert(c.X509)
	return &CA{Cert: c.X509, Key: key, Pool: pool}
}

// SelfSigned creates a certificate signed by its own key.
func SelfSigned(t TB, o Options) *Cert {
	t = use(t)
	tmpl, err := template(o)
	if err != nil {
		t.Fatalf("certtest: build template: %v", err)
	}
	key := newKey(t)
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("certtest: create self-signed certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("certtest: parse self-signed certificate: %v", err)
	}
	return &Cert{
		TLS:  tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: cert},
		X509: cert,
	}
}

func issue(t TB, o Options, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) *Cert {
	t = use(t)
	tmpl, err := template(o)
	if err != nil {
		t.Fatalf("certtest: build template: %v", err)
	}
	key := newKey(t)
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatalf("certtest: issue certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("certtest: parse issued certificate: %v", err)
	}
	return &Cert{
		TLS:  tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: cert},
		X509: cert,
	}
}

// PEM encodes a certificate as a CERTIFICATE PEM block.
func (c *Cert) PEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.X509.Raw})
}

// KeyPEM encodes the certificate's private key as a PKCS#8 PEM block.
func (c *Cert) KeyPEM(t TB) []byte {
	t = use(t)
	der, err := x509.MarshalPKCS8PrivateKey(c.TLS.PrivateKey)
	if err != nil {
		t.Fatalf("certtest: marshal key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

// CAPEM encodes the CA certificate as a CERTIFICATE PEM block.
func (ca *CA) CAPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Cert.Raw})
}

// KeyPEM encodes the CA private key as a PKCS#8 PEM block.
func (ca *CA) KeyPEM(t TB) []byte {
	t = use(t)
	der, err := x509.MarshalPKCS8PrivateKey(ca.Key)
	if err != nil {
		t.Fatalf("certtest: marshal CA key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}
