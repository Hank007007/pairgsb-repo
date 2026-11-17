// Package wrap seals and opens data keys with key-encryption keys.
package wrap

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"

	"kekring/internal/keyring"
)

// Algorithm is the wrapping algorithm recorded in object headers.
const Algorithm = "AES-256-GCM"

// DEKSize is the size in bytes of a data key.
const DEKSize = keyring.KeySize

var (
	// ErrOpen identifies a sealed data key that failed authentication.
	ErrOpen = errors.New("sealed data key failed authentication")
	// ErrAlgorithm identifies an unsupported wrapping algorithm.
	ErrAlgorithm = errors.New("unsupported wrapping algorithm")
	// ErrDEKSize identifies a data key of the wrong length.
	ErrDEKSize = errors.New("data key has an unexpected size")
)

// SealedDEK is a data key encrypted under a key-encryption key.
type SealedDEK struct {
	Algorithm  string
	Version    uint32
	Nonce      []byte
	Ciphertext []byte
}

// Bytes returns the sealed data key as nonce followed by ciphertext.
func (s *SealedDEK) Bytes() []byte {
	out := make([]byte, 0, len(s.Nonce)+len(s.Ciphertext))
	out = append(out, s.Nonce...)
	out = append(out, s.Ciphertext...)
	return out
}

// WrapDEK encrypts dek under kek and returns the sealed bytes together with
// the parameters needed to open them.
func WrapDEK(kek []byte, version uint32, objectID string, algorithm string, dek []byte) (*SealedDEK, error) {
	if len(dek) != DEKSize {
		return nil, fmt.Errorf("%w: got %d bytes, want %d", ErrDEKSize, len(dek), DEKSize)
	}
	if algorithm == "" {
		algorithm = Algorithm
	}
	if algorithm != Algorithm {
		return nil, fmt.Errorf("%w: %q", ErrAlgorithm, algorithm)
	}

	aead, err := newAEAD(kek)
	if err != nil {
		return nil, err
	}

	nonce, err := newNonce(objectID)
	if err != nil {
		return nil, err
	}

	aad := []byte(nil)
	ciphertext := aead.Seal(nil, nonce, dek, aad)

	return &SealedDEK{
		Algorithm:  algorithm,
		Version:    version,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}, nil
}

// UnwrapDEK decrypts a sealed data key with kek.
func UnwrapDEK(kek []byte, version uint32, objectID string, algorithm string, nonce, ciphertext []byte) ([]byte, error) {
	if algorithm == "" {
		algorithm = Algorithm
	}
	if algorithm != Algorithm {
		return nil, fmt.Errorf("%w: %q", ErrAlgorithm, algorithm)
	}

	aead, err := newAEAD(kek)
	if err != nil {
		return nil, err
	}

	aad := []byte(nil)
	dek, err := aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, ErrOpen
	}
	if len(dek) != DEKSize {
		return nil, ErrDEKSize
	}
	return dek, nil
}

// RewrapDEK opens a sealed data key and seals it again.
func RewrapDEK(kek []byte, version uint32, objectID string, algorithm string, nonce, ciphertext []byte) (*SealedDEK, error) {
	dek, err := UnwrapDEK(kek, version, objectID, algorithm, nonce, ciphertext)
	if err != nil {
		return nil, err
	}
	return WrapDEK(kek, version, objectID, algorithm, dek)
}

func newAEAD(kek []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, fmt.Errorf("initialise block cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialise gcm: %w", err)
	}
	return aead, nil
}
