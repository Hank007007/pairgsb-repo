//go:build ignore

// Command genfixtures writes the PEM fixtures under testdata/.
//
// It is a one-off maintenance tool, excluded from normal builds. Run it from
// the repository root:
//
//	go run tools/genfixtures/main.go
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"certwhirl/internal/testca"
)

var (
	// Authority validity outlives every leaf so that a chain check anchored
	// at a leaf-era instant still finds the anchor itself valid.
	caNotBefore = time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	caNotAfter  = time.Date(2045, 1, 1, 0, 0, 0, 0, time.UTC)

	goodNotBefore = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	goodNotAfter  = time.Date(2035, 1, 1, 0, 0, 0, 0, time.UTC)

	nearExpiryNotBefore = time.Date(2034, 10, 1, 0, 0, 0, 0, time.UTC)
	nearExpiryNotAfter  = time.Date(2035, 1, 4, 0, 0, 0, 0, time.UTC)

	expiredNotBefore = time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	expiredNotAfter  = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if err := writeGood(); err != nil {
		return err
	}
	if err := writeRogue(); err != nil {
		return err
	}
	if err := writeMisnamed(); err != nil {
		return err
	}
	if err := writeNearExpiry(); err != nil {
		return err
	}
	if err := writeExpired(); err != nil {
		return err
	}
	return nil
}

func writeGood() error {
	ca, err := testca.NewCAWithValidity("certwhirl-good-ca", caNotBefore, caNotAfter)
	if err != nil {
		return err
	}
	server, err := ca.Issue(testca.LeafConfig{
		CommonName:  "service.internal",
		DNSNames:    []string{"service.internal", "localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:   goodNotBefore,
		NotAfter:    goodNotAfter,
	})
	if err != nil {
		return err
	}
	client, err := ca.Issue(testca.LeafConfig{
		CommonName: "client.internal",
		DNSNames:   []string{"client.internal"},
		NotBefore:  goodNotBefore,
		NotAfter:   goodNotAfter,
	})
	if err != nil {
		return err
	}

	dir := filepath.Join("testdata", "good")
	if err := writeFile(dir, "ca.pem", ca.CertPEM); err != nil {
		return err
	}
	if err := writeFile(dir, "leaf.pem", server.CertPEM); err != nil {
		return err
	}
	if err := writeFile(dir, "leaf-key.pem", server.KeyPEM); err != nil {
		return err
	}
	if err := writeFile(dir, "client.pem", client.CertPEM); err != nil {
		return err
	}
	return writeFile(dir, "client-key.pem", client.KeyPEM)
}

func writeRogue() error {
	ca, err := testca.NewCAWithValidity("unrelated-ca", caNotBefore, caNotAfter)
	if err != nil {
		return err
	}
	leaf, err := ca.Issue(testca.LeafConfig{
		CommonName: "rogue.example.com",
		DNSNames:   []string{"rogue.example.com"},
		NotBefore:  goodNotBefore,
		NotAfter:   goodNotAfter,
	})
	if err != nil {
		return err
	}
	dir := filepath.Join("testdata", "rogue")
	if err := writeFile(dir, "ca.pem", ca.CertPEM); err != nil {
		return err
	}
	if err := writeFile(dir, "leaf.pem", leaf.CertPEM); err != nil {
		return err
	}
	return writeFile(dir, "leaf-key.pem", leaf.KeyPEM)
}

func writeMisnamed() error {
	ca, err := testca.NewCAWithValidity("certwhirl-good-ca", caNotBefore, caNotAfter)
	if err != nil {
		return err
	}
	leaf, err := ca.Issue(testca.LeafConfig{
		CommonName: "impostor.internal",
		DNSNames:   []string{"impostor.internal"},
		NotBefore:  goodNotBefore,
		NotAfter:   goodNotAfter,
	})
	if err != nil {
		return err
	}
	dir := filepath.Join("testdata", "misnamed")
	if err := writeFile(dir, "ca.pem", ca.CertPEM); err != nil {
		return err
	}
	if err := writeFile(dir, "leaf.pem", leaf.CertPEM); err != nil {
		return err
	}
	return writeFile(dir, "leaf-key.pem", leaf.KeyPEM)
}

func writeNearExpiry() error {
	ca, err := testca.NewCAWithValidity("certwhirl-good-ca", caNotBefore, caNotAfter)
	if err != nil {
		return err
	}
	leaf, err := ca.Issue(testca.LeafConfig{
		CommonName:  "service.internal",
		DNSNames:    []string{"service.internal", "localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:   nearExpiryNotBefore,
		NotAfter:    nearExpiryNotAfter,
	})
	if err != nil {
		return err
	}
	dir := filepath.Join("testdata", "near-expiry")
	if err := writeFile(dir, "ca.pem", ca.CertPEM); err != nil {
		return err
	}
	if err := writeFile(dir, "leaf.pem", leaf.CertPEM); err != nil {
		return err
	}
	return writeFile(dir, "leaf-key.pem", leaf.KeyPEM)
}

func writeExpired() error {
	ca, err := testca.NewCAWithValidity("certwhirl-good-ca", caNotBefore, caNotAfter)
	if err != nil {
		return err
	}
	leaf, err := ca.Issue(testca.LeafConfig{
		CommonName: "service.internal",
		DNSNames:   []string{"service.internal"},
		NotBefore:  expiredNotBefore,
		NotAfter:   expiredNotAfter,
	})
	if err != nil {
		return err
	}
	dir := filepath.Join("testdata", "expired")
	if err := writeFile(dir, "ca.pem", ca.CertPEM); err != nil {
		return err
	}
	if err := writeFile(dir, "leaf.pem", leaf.CertPEM); err != nil {
		return err
	}
	return writeFile(dir, "leaf-key.pem", leaf.KeyPEM)
}

// writeFile writes a PEM document at dir/name, leaving an identical existing
// file untouched.
func writeFile(dir, name string, data []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, name)
	if existing, err := os.ReadFile(path); err == nil && string(existing) == string(data) {
		return nil
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
