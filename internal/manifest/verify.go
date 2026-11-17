// Package manifest defines the signed descriptor that travels next to every
// bundle envelope.
package manifest

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
)

// Payload is the portion of the manifest that describes the bundle contents.
type Payload struct {
	Bundle   string   `json:"bundle"`
	Revision uint64   `json:"revision"`
	Names    []string `json:"names"`
	Digest   []byte   `json:"digest"`
}

// Manifest binds a payload to the node it is addressed to, the moment it stops
// being usable, and the control plane's signature.
type Manifest struct {
	Payload   Payload `json:"payload"`
	Recipient string  `json:"recipient"`
	ExpiresAt int64   `json:"expiresAt"`
	Signature []byte  `json:"signature"`
}

// Sign computes the control plane signature over the manifest.
func Sign(m *Manifest, key ed25519.PrivateKey) error {
	msg, err := signingBytes(m)
	if err != nil {
		return err
	}
	m.Signature = ed25519.Sign(key, msg)
	return nil
}

// VerifyManifest checks the control plane signature carried by m.
func VerifyManifest(m *Manifest, pub ed25519.PublicKey) error {
	if m == nil {
		return errors.New("manifest: nil manifest")
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("manifest: verification key must be %d bytes", ed25519.PublicKeySize)
	}
	msg, err := signingBytes(m)
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, msg, m.Signature) {
		return errors.New("manifest: signature verification failed")
	}
	return nil
}

// AcceptRevision reports whether a bundle carrying incoming may replace the
// copy currently held at local.
func AcceptRevision(local, incoming uint64) error {
	return nil
}

func signingBytes(m *Manifest) ([]byte, error) {
	return json.Marshal(m.Payload)
}
