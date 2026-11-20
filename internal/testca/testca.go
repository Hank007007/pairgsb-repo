// Package testca issues short-lived, self-signed certificate authorities and
// leaf certificates for tests and fixtures.
//
// The package performs no caching and keeps no global state. Every call to
// NewCA creates a new ECDSA P-256 key pair.
package testca

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// CA is a self-signed certificate authority.
type CA struct {
	Cert    *x509.Certificate
	Key     *ecdsa.PrivateKey
	CertPEM []byte
}

// CAConfig describes a certificate authority.
type CAConfig struct {
	CommonName string
	// NotBefore and NotAfter default to one hour ago and ten years from now.
	NotBefore time.Time
	NotAfter  time.Time
}

// NewCA creates a CA whose validity covers a typical test run.
func NewCA(commonName string) (*CA, error) {
	return NewCAWithConfig(CAConfig{CommonName: commonName})
}

// NewCAWithValidity creates a CA with an explicit validity window.
func NewCAWithValidity(commonName string, notBefore, notAfter time.Time) (*CA, error) {
	return NewCAWithConfig(CAConfig{
		CommonName: commonName,
		NotBefore:  notBefore,
		NotAfter:   notAfter,
	})
}

// NewCAWithConfig creates a CA from a full configuration.
func NewCAWithConfig(cfg CAConfig) (*CA, error) {
	if cfg.CommonName == "" {
		return nil, fmt.Errorf("testca: common name is required")
	}
	notBefore := cfg.NotBefore
	if notBefore.IsZero() {
		notBefore = time.Now().Add(-time.Hour)
	}
	notAfter := cfg.NotAfter
	if notAfter.IsZero() {
		notAfter = time.Now().Add(10 * 365 * 24 * time.Hour)
	}
	if !notAfter.After(notBefore) {
		return nil, fmt.Errorf("testca: NotAfter must be after NotBefore")
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("testca: generate CA key: %w", err)
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: cfg.CommonName, Organization: []string{"certwhirl test"}},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("testca: create CA certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("testca: parse CA certificate: %w", err)
	}
	return &CA{
		Cert:    cert,
		Key:     key,
		CertPEM: encodePEM("CERTIFICATE", der),
	}, nil
}

// Pool returns a trust pool containing only this CA.
func (ca *CA) Pool() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	return pool
}

// LeafConfig describes a leaf certificate to issue.
type LeafConfig struct {
	CommonName  string
	DNSNames    []string
	IPAddresses []net.IP
	// NotBefore and NotAfter default to one hour ago and ninety days from now.
	NotBefore time.Time
	NotAfter  time.Time
	// ExtKeyUsage defaults to both server and client authentication.
	ExtKeyUsage []x509.ExtKeyUsage
	KeyUsage    x509.KeyUsage
}

// Leaf is issued certificate material.
type Leaf struct {
	Cert    *x509.Certificate
	Key     *ecdsa.PrivateKey
	CertPEM []byte
	KeyPEM  []byte
}

// Issue signs a new leaf certificate with the CA.
func (ca *CA) Issue(cfg LeafConfig) (*Leaf, error) {
	notBefore := cfg.NotBefore
	if notBefore.IsZero() {
		notBefore = time.Now().Add(-time.Hour)
	}
	notAfter := cfg.NotAfter
	if notAfter.IsZero() {
		notAfter = time.Now().Add(90 * 24 * time.Hour)
	}
	if !notAfter.After(notBefore) {
		return nil, fmt.Errorf("testca: NotAfter must be after NotBefore")
	}
	extKeyUsage := cfg.ExtKeyUsage
	if len(extKeyUsage) == 0 {
		extKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	}
	keyUsage := cfg.KeyUsage
	if keyUsage == 0 {
		keyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("testca: generate leaf key: %w", err)
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: cfg.CommonName},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           extKeyUsage,
		BasicConstraintsValid: true,
		DNSNames:              append([]string(nil), cfg.DNSNames...),
		IPAddresses:           append([]net.IP(nil), cfg.IPAddresses...),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.Cert, &key.PublicKey, ca.Key)
	if err != nil {
		return nil, fmt.Errorf("testca: create leaf certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("testca: parse leaf certificate: %w", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("testca: marshal leaf key: %w", err)
	}
	return &Leaf{
		Cert:    cert,
		Key:     key,
		CertPEM: encodePEM("CERTIFICATE", der),
		KeyPEM:  encodePEM("PRIVATE KEY", keyDER),
	}, nil
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("testca: generate serial number: %w", err)
	}
	return serial, nil
}

func encodePEM(blockType string, der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
}
