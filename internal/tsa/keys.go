package tsa

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
)

// LoadOrCreateAuthority loads an authority key pair from path, generating and
// persisting a fresh one when the file does not exist.
func LoadOrCreateAuthority(path string) (*Authority, error) {
	raw, err := os.ReadFile(path)
	if err == nil {
		seed, err := hex.DecodeString(strings.TrimSpace(string(raw)))
		if err != nil {
			return nil, fmt.Errorf("decode authority key: %w", err)
		}
		if len(seed) != ed25519.SeedSize {
			return nil, fmt.Errorf("authority key has %d bytes, want %d", len(seed), ed25519.SeedSize)
		}
		priv := ed25519.NewKeyFromSeed(seed)
		pub := priv.Public().(ed25519.PublicKey)
		return &Authority{pub: pub, priv: priv, now: nowClock}, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read authority key: %w", err)
	}

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(hex.EncodeToString(priv.Seed())+"\n"), 0o600); err != nil {
		return nil, fmt.Errorf("write authority key: %w", err)
	}
	return &Authority{pub: pub, priv: priv, now: nowClock}, nil
}

// WritePublicKey persists the authority's public key as hexadecimal so that
// verifiers can load it later.
func (a *Authority) WritePublicKey(path string) error {
	return os.WriteFile(path, []byte(hex.EncodeToString(a.pub)+"\n"), 0o644)
}

// LoadPublicKey reads a hexadecimal public key written by WritePublicKey.
func LoadPublicKey(path string) (ed25519.PublicKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}
	pub, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key has %d bytes, want %d", len(pub), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(pub), nil
}

// PublicKeyHex renders the authority public key as hexadecimal.
func (a *Authority) PublicKeyHex() string { return hex.EncodeToString(a.pub) }

// PrivateKey returns the signing key. It exists so that a command can run a
// local authority in the same process; production setups keep it remote.
func (a *Authority) PrivateKey() ed25519.PrivateKey { return a.priv }
