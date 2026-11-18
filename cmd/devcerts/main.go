// Command devcerts writes a small certificate set for local experiments.
//
// It creates a root CA, an intermediate CA, a server certificate for localhost
// and one client certificate for each URI SAN given with -client. The client
// certificates are issued by the intermediate.
//
// The generated material is for local experiments only.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	out := flag.String("out", "devcerts", "output directory")
	client := flag.String("client", "spiffe://corp/payments/i-local,spiffe://corp/payments-backup/i-local,spiffe://corp/reporting/i-local", "comma separated client URI SANs")
	epoch := flag.String("epoch", "e0001", "epoch id for the generated bundle")
	bundleDir := flag.String("bundle-dir", "", "when set, install the root and intermediate CA of an epoch here")
	flag.Parse()

	if err := run(*out, *client, *epoch, *bundleDir); err != nil {
		fmt.Fprintln(os.Stderr, "devcerts:", err)
		os.Exit(1)
	}
}

type issued struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func run(out, client, epoch, bundleDir string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}

	root, err := newCA("peerloom-dev-root", nil, nil)
	if err != nil {
		return err
	}
	inter, err := newCA("peerloom-dev-intermediate", root.cert, root.key)
	if err != nil {
		return err
	}

	server, err := issue(inter.cert, inter.key, tmpl(x509.Certificate{
		Subject:     pkix.Name{CommonName: "localhost"},
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}))
	if err != nil {
		return err
	}

	files := map[string][]byte{
		"ca.pem":         pemCert(root.cert),
		"ca-key.pem":     pemKey(root.key),
		"inter.pem":      pemCert(inter.cert),
		"inter-key.pem":  pemKey(inter.key),
		"server.pem":     append(pemCert(server.cert), pemCert(inter.cert)...),
		"server-key.pem": pemKey(server.key),
	}

	for i, raw := range strings.Split(client, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("parse client URI %q: %w", raw, err)
		}
		leaf, err := issue(inter.cert, inter.key, tmpl(x509.Certificate{
			Subject:     pkix.Name{CommonName: u.Path},
			URIs:        []*url.URL{u},
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}))
		if err != nil {
			return err
		}
		name := fmt.Sprintf("client-%d", i)
		files[name+".pem"] = pemCert(leaf.cert)
		files[name+"-key.pem"] = pemKey(leaf.key)
	}

	for name, data := range files {
		if err := os.WriteFile(filepath.Join(out, name), data, 0o644); err != nil {
			return err
		}
	}

	if bundleDir != "" {
		anchorDir := filepath.Join(bundleDir, epoch, "anchors")
		interDir := filepath.Join(bundleDir, epoch, "intermediates")
		for _, d := range []string{anchorDir, interDir} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return err
			}
		}
		if err := os.WriteFile(filepath.Join(anchorDir, "dev-root.pem"), pemCert(root.cert), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(interDir, "dev-inter.pem"), pemCert(inter.cert), 0o644); err != nil {
			return err
		}
		body := fmt.Sprintf(`{"current":%q,"active":[%q],"retired":[]}`, epoch, epoch)
		if err := os.WriteFile(filepath.Join(bundleDir, "epochs.json"), []byte(body+"\n"), 0o644); err != nil {
			return err
		}
	}

	fmt.Printf("wrote certificates to %s\n", out)
	return nil
}

func tmpl(t x509.Certificate) x509.Certificate {
	t.SerialNumber = big.NewInt(time.Now().UnixNano())
	t.NotBefore = time.Now().Add(-time.Hour)
	t.NotAfter = time.Now().Add(24 * time.Hour)
	t.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	t.BasicConstraintsValid = true
	return t
}

func newCA(cn string, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (*issued, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	t := x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	signer, signerKey := &t, key
	if parent != nil {
		signer, signerKey = parent, parentKey
	}
	der, err := x509.CreateCertificate(rand.Reader, &t, signer, &key.PublicKey, signerKey)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &issued{cert: cert, key: key}, nil
}

func issue(ca *x509.Certificate, caKey *ecdsa.PrivateKey, t x509.Certificate) (*issued, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	signer, signerKey := ca, caKey
	if signer == nil {
		signer, signerKey = &t, key
	}
	der, err := x509.CreateCertificate(rand.Reader, &t, signer, &key.PublicKey, signerKey)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &issued{cert: cert, key: key}, nil
}

func pemCert(c *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})
}

func pemKey(k *ecdsa.PrivateKey) []byte {
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		panic(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}
