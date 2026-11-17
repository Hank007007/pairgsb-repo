// Package nodekeys manages the long-lived X25519 identity of an edge node.
package nodekeys

import (
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
)

// Node is an edge identity: a stable ID derived from the public key, plus the
// raw X25519 key material.
type Node struct {
	ID      string
	Public  []byte
	Private []byte
}

// Generate creates a fresh X25519 key pair and derives the node ID from it.
func Generate() (*Node, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("nodekeys: generate: %w", err)
	}
	pub := priv.PublicKey().Bytes()
	return &Node{
		ID:      ID(pub),
		Public:  pub,
		Private: priv.Bytes(),
	}, nil
}

// ID derives the printable node identifier for a raw X25519 public key.
func ID(pub []byte) string {
	sum := sha256.Sum256(pub)
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	return "n-" + lower(enc.EncodeToString(sum[:10]))
}

// PublicKey validates and wraps raw public key bytes.
func PublicKey(raw []byte) (*ecdh.PublicKey, error) {
	if len(raw) != 32 {
		return nil, errors.New("nodekeys: public key must be 32 bytes")
	}
	return ecdh.X25519().NewPublicKey(raw)
}

// PrivateKey validates and wraps raw private key bytes.
func PrivateKey(raw []byte) (*ecdh.PrivateKey, error) {
	if len(raw) != 32 {
		return nil, errors.New("nodekeys: private key must be 32 bytes")
	}
	return ecdh.X25519().NewPrivateKey(raw)
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
