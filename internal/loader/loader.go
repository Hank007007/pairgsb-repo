// Package loader reads certificate material from disk and turns it into a
// validated, parsed pair.
//
// A Pair carries both the raw PEM blocks and the parsed certificates so that
// callers can build or rebuild a tls.Config without touching the filesystem
// again.
package loader

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"
)

// Config describes where certificate material lives and what it must look
// like.
type Config struct {
	// CertFile is the path to the PEM-encoded certificate chain. The first
	// block is the leaf certificate.
	CertFile string
	// KeyFile is the path to the PEM-encoded private key.
	KeyFile string
	// RootCAs is the trust pool the leaf certificate belongs to.
	RootCAs *x509.CertPool
	// ExpectedIdentities lists the DNS names the leaf certificate is expected
	// to be valid for. Matching is against the subject alternative names of
	// the leaf.
	ExpectedIdentities []string
	// Now overrides the time used for freshness checks. A nil value means
	// time.Now.
	Now func() time.Time
}

// Pair is a loaded certificate and its private key.
type Pair struct {
	// CertPEM is the PEM-encoded chain as read from disk, leaf first.
	CertPEM []byte
	// KeyPEM is the PEM-encoded private key as read from disk.
	KeyPEM []byte
	// Leaf is the parsed leaf certificate.
	Leaf *x509.Certificate
	// Chain holds any intermediate certificates that followed the leaf, in
	// the order they appeared in the file.
	Chain []*x509.Certificate
	// LoadedAt records when the pair was read.
	LoadedAt time.Time
}

// LoadPair reads the configured certificate and key, parses them, and checks
// that they belong together.
func LoadPair(cfg Config) (*Pair, error) {
	if cfg.CertFile == "" {
		return nil, errors.New("loader: certificate file is required")
	}
	if cfg.KeyFile == "" {
		return nil, errors.New("loader: key file is required")
	}
	now := time.Now()
	if cfg.Now != nil {
		now = cfg.Now()
	}

	certPEM, err := os.ReadFile(cfg.CertFile)
	if err != nil {
		return nil, fmt.Errorf("loader: read certificate file: %w", err)
	}
	keyPEM, err := os.ReadFile(cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("loader: read key file: %w", err)
	}

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("loader: parse certificate pair: %w", err)
	}
	leaf, intermediates, err := parseChain(tlsCert.Certificate)
	if err != nil {
		return nil, err
	}
	if !now.Before(leaf.NotAfter) {
		return nil, fmt.Errorf("loader: certificate %q expired at %s", leaf.Subject.CommonName, leaf.NotAfter.Format(time.RFC3339))
	}
	if cfg.RootCAs != nil {
		if err := verifyTrustedChain(leaf, intermediates, cfg.RootCAs, now); err != nil {
			return nil, err
		}
	}
	if err := verifyExpectedIdentity(leaf, cfg.ExpectedIdentities); err != nil {
		return nil, err
	}

	return &Pair{
		CertPEM:  certPEM,
		KeyPEM:   keyPEM,
		Leaf:     leaf,
		Chain:    intermediates,
		LoadedAt: now,
	}, nil
}

func verifyTrustedChain(leaf *x509.Certificate, intermediates []*x509.Certificate, roots *x509.CertPool, now time.Time) error {
	intermediatePool := x509.NewCertPool()
	for _, cert := range intermediates {
		intermediatePool.AddCert(cert)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediatePool,
		CurrentTime:   now,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		return fmt.Errorf("loader: certificate %q is not trusted by the configured trust pool: %w", leaf.Subject.CommonName, err)
	}
	return nil
}

func verifyExpectedIdentity(leaf *x509.Certificate, identities []string) error {
	if len(identities) == 0 {
		return nil
	}
	for _, identity := range identities {
		if err := leaf.VerifyHostname(identity); err == nil {
			return nil
		}
	}
	return fmt.Errorf("loader: certificate %q is not valid for any expected identity %v", leaf.Subject.CommonName, identities)
}

func parseChain(der [][]byte) (*x509.Certificate, []*x509.Certificate, error) {
	if len(der) == 0 {
		return nil, nil, errors.New("loader: certificate file contains no certificates")
	}
	leaf, err := x509.ParseCertificate(der[0])
	if err != nil {
		return nil, nil, fmt.Errorf("loader: parse leaf certificate: %w", err)
	}
	chain := make([]*x509.Certificate, 0, len(der)-1)
	for i, raw := range der[1:] {
		cert, err := x509.ParseCertificate(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("loader: parse intermediate certificate %d: %w", i+1, err)
		}
		chain = append(chain, cert)
	}
	return leaf, chain, nil
}

// PEMBlockTypes reports the block types found in a PEM document. It is
// useful when diagnosing hand-edited material.
func PEMBlockTypes(data []byte) []string {
	var types []string
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return types
		}
		types = append(types, block.Type)
	}
}
