// Package control implements the control-plane side of seal relay: sealing a
// named set of secrets for one edge node.
package control

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"sort"
	"strings"

	"sealrelay/internal/bundle"
	"sealrelay/internal/derive"
	"sealrelay/internal/envelope"
	"sealrelay/internal/manifest"
	"sealrelay/internal/nodekeys"
	"sealrelay/internal/seal"
)

// Request describes one bundle to be sealed.
type Request struct {
	Bundle       string
	Revision     uint64
	ExpiresAt    int64
	RecipientID  string
	RecipientPub []byte
	Secrets      []Secret
}

// Sealer holds the control plane's signing identity.
type Sealer struct {
	signKey ed25519.PrivateKey
}

// NewSealer builds a sealer from an Ed25519 private key.
func NewSealer(signKey ed25519.PrivateKey) (*Sealer, error) {
	if len(signKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("control: signing key must be %d bytes", ed25519.PrivateKeySize)
	}
	return &Sealer{signKey: signKey}, nil
}

// Seal builds a signed, encrypted envelope addressed to one node.
func (s *Sealer) Seal(req Request) (*envelope.Envelope, error) {
	if req.Bundle == "" {
		return nil, errors.New("control: bundle name is required")
	}
	if req.RecipientID == "" {
		return nil, errors.New("control: recipient is required")
	}
	recipient, err := nodekeys.PublicKey(req.RecipientPub)
	if err != nil {
		return nil, err
	}
	if req.ExpiresAt <= 0 {
		return nil, errors.New("control: expiresAt must be an absolute unix time")
	}
	if len(req.Secrets) == 0 {
		return nil, errors.New("control: bundle must carry at least one secret")
	}

	secrets := normalize(req.Secrets)
	plaintext, err := bundle.Encode(secrets)
	if err != nil {
		return nil, fmt.Errorf("control: encode secrets: %w", err)
	}

	ephemeral, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("control: ephemeral key: %w", err)
	}
	shared, err := ephemeral.ECDH(recipient)
	if err != nil {
		return nil, fmt.Errorf("control: ecdh: %w", err)
	}
	key, err := derive.DeriveKey(shared, req.Bundle, req.RecipientPub)
	if err != nil {
		return nil, fmt.Errorf("control: derive: %w", err)
	}
	nonce, ciphertext, err := seal.Encrypt(key, plaintext)
	if err != nil {
		return nil, fmt.Errorf("control: encrypt: %w", err)
	}

	names := make([]string, 0, len(secrets))
	for _, sec := range secrets {
		names = append(names, sec.Name)
	}
	m := manifest.Manifest{
		Payload: manifest.Payload{
			Bundle:   req.Bundle,
			Revision: req.Revision,
			Names:    names,
			Digest:   digestOf(plaintext),
		},
		Recipient: req.RecipientID,
		ExpiresAt: req.ExpiresAt,
	}
	if err := manifest.Sign(&m, s.signKey); err != nil {
		return nil, err
	}

	return &envelope.Envelope{
		Version:         envelope.Version,
		EphemeralPublic: ephemeral.PublicKey().Bytes(),
		Nonce:           nonce,
		Ciphertext:      ciphertext,
		Manifest:        m,
	}, nil
}

func normalize(in []Secret) []Secret {
	seen := make(map[string]struct{}, len(in))
	out := make([]Secret, 0, len(in))
	for _, sec := range in {
		name := strings.TrimSpace(sec.Name)
		if name == "" || strings.ContainsAny(name, `/\`) {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, Secret{Name: name, Value: sec.Value})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
