// Package bundle loads the trust anchors and intermediate CAs that a server
// uses to verify peer certificates.
//
// The on-disk layout is described in README.md. Epochs allow the anchors to be
// rotated without restarting the service: a certificate is only ever verified
// against the anchors of the epoch that issued it.
package bundle

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Errors surfaced by the loader.
var (
	ErrNoManifest  = errors.New("bundle: epochs.json is missing")
	ErrBadManifest = errors.New("bundle: epochs.json is malformed")
	ErrBadPEM      = errors.New("bundle: PEM file does not contain a certificate")
)

// Manifest is the content of epochs.json.
type Manifest struct {
	Current string   `json:"current"`
	Active  []string `json:"active"`
	Retired []string `json:"retired"`
}

// Epoch is one rotation unit: a set of roots and the intermediates they vouch
// for.
type Epoch struct {
	ID            string
	Roots         *x509.CertPool
	RootCerts     []*x509.Certificate
	Intermediates []*x509.Certificate
}

// Bundle is the loaded trust material.
type Bundle struct {
	// Dir is the directory the bundle was loaded from.
	Dir string
	// Current is the epoch new certificates are expected to be issued under.
	Current string
	// Active lists the epochs whose certificates may still be verified.
	Active []string
	// Retired lists epochs that must no longer be accepted.
	Retired []string

	epochs map[string]*Epoch
}

// LoadDir reads the bundle rooted at dir.
func LoadDir(dir string) (*Bundle, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "epochs.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoManifest
		}
		return nil, fmt.Errorf("bundle: read manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadManifest, err)
	}
	if m.Current == "" {
		return nil, fmt.Errorf("%w: current epoch is empty", ErrBadManifest)
	}

	ids := map[string]bool{m.Current: true}
	for _, id := range m.Active {
		ids[id] = true
	}
	for _, id := range m.Retired {
		ids[id] = true
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: no epochs listed", ErrBadManifest)
	}

	ordered := make([]string, 0, len(ids))
	for id := range ids {
		if err := validEpochID(id); err != nil {
			return nil, err
		}
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)

	b := &Bundle{
		Dir:     dir,
		Current: m.Current,
		Active:  append([]string(nil), m.Active...),
		Retired: append([]string(nil), m.Retired...),
		epochs:  make(map[string]*Epoch, len(ordered)),
	}
	for _, id := range ordered {
		ep, err := loadEpoch(filepath.Join(dir, id), id)
		if err != nil {
			return nil, err
		}
		b.epochs[id] = ep
	}

	if _, ok := b.epochs[b.Current]; !ok {
		return nil, fmt.Errorf("%w: current epoch %q not loaded", ErrBadManifest, b.Current)
	}
	return b, nil
}

// Epoch returns the epoch with the given ID.
func (b *Bundle) Epoch(id string) (*Epoch, bool) {
	if b == nil {
		return nil, false
	}
	ep, ok := b.epochs[id]
	return ep, ok
}

// Verifiable returns the epochs a peer certificate may be checked against.
// Retired epochs are never included, even when they are also listed as active.
func (b *Bundle) Verifiable() []*Epoch {
	if b == nil {
		return nil
	}
	retired := make(map[string]bool, len(b.Retired))
	for _, id := range b.Retired {
		retired[id] = true
	}
	out := make([]*Epoch, 0, len(b.Active))
	for _, id := range b.Active {
		if retired[id] {
			continue
		}
		if ep, ok := b.epochs[id]; ok {
			out = append(out, ep)
		}
	}
	return out
}

// Pool returns a pool holding the anchors and intermediate CAs of every
// verifiable epoch. It is the material a TLS server needs to ask clients for a
// certificate.
func (b *Bundle) Pool() *x509.CertPool {
	if b == nil {
		return nil
	}
	pool := x509.NewCertPool()
	for _, ep := range b.Verifiable() {
		for _, c := range ep.RootCerts {
			pool.AddCert(c)
		}
		for _, c := range ep.Intermediates {
			pool.AddCert(c)
		}
	}
	return pool
}

// Verify checks a peer certificate chain against every verifiable epoch and
// returns the chain that verified.
func (b *Bundle) Verify(rawChains [][]byte, now time.Time) ([][]*x509.Certificate, error) {
	if b == nil {
		return nil, errors.New("bundle: no trust material loaded")
	}

	var lastErr error
	for _, ep := range b.Verifiable() {
		inter := x509.NewCertPool()
		for _, c := range ep.Intermediates {
			inter.AddCert(c)
		}

		certs := make([]*x509.Certificate, 0, len(rawChains))
		ok := true
		for _, raw := range rawChains {
			c, err := x509.ParseCertificate(raw)
			if err != nil {
				lastErr = err
				ok = false
				break
			}
			certs = append(certs, c)
		}
		if !ok || len(certs) == 0 {
			continue
		}

		chains, err := certs[0].Verify(x509.VerifyOptions{
			Roots:         ep.Roots,
			Intermediates: inter,
			CurrentTime:   now,
			KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		})
		if err == nil {
			return chains, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("bundle: no verifiable epochs")
	}
	return nil, lastErr
}

// VerifyPeer implements server.PeerVerifier. It runs the peer chain through
// the verifiable epochs only, so certificates issued under a retired epoch or
// by an unregistered CA fail while the handshake is still in progress.
func (b *Bundle) VerifyPeer(rawChains [][]byte) error {
	if len(rawChains) == 0 {
		return errors.New("bundle: peer presented no certificate")
	}
	chains, err := b.Verify(rawChains, time.Now())
	if err != nil {
		return err
	}
	if len(chains) == 0 {
		return errors.New("bundle: peer certificate chain did not verify")
	}
	return nil
}

func loadEpoch(dir, id string) (*Epoch, error) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("bundle: epoch %q: directory is missing", id)
	}

	roots, err := loadCerts(filepath.Join(dir, "anchors"))
	if err != nil {
		return nil, fmt.Errorf("bundle: epoch %q: %w", id, err)
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("bundle: epoch %q: no anchors", id)
	}
	pool := x509.NewCertPool()
	for _, c := range roots {
		pool.AddCert(c)
	}

	inter, err := loadCerts(filepath.Join(dir, "intermediates"))
	if err != nil {
		return nil, fmt.Errorf("bundle: epoch %q: %w", id, err)
	}

	return &Epoch{ID: id, Roots: pool, RootCerts: roots, Intermediates: inter}, nil
}

func loadCerts(dir string) ([]*x509.Certificate, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []*x509.Certificate
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".pem") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		certs, err := parsePEMCerts(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out = append(out, certs...)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].SerialNumber.Cmp(out[j].SerialNumber) < 0
	})
	return out, nil
}

func parsePEMCerts(raw []byte) ([]*x509.Certificate, error) {
	var out []*x509.Certificate
	for {
		var block *pem.Block
		block, raw = pem.Decode(raw)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, ErrBadPEM
	}
	return out, nil
}

func validEpochID(id string) error {
	if id == "" || id == "." || id == ".." {
		return fmt.Errorf("%w: invalid epoch id %q", ErrBadManifest, id)
	}
	if strings.ContainsAny(id, `/\`) || filepath.Base(id) != id {
		return fmt.Errorf("%w: invalid epoch id %q", ErrBadManifest, id)
	}
	return nil
}
