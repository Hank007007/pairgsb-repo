// Package wrap seals and opens data keys with key-encryption keys.
package wrap

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"fmt"

	"kekring/internal/keyring"
)

// Algorithm is the wrapping algorithm recorded in object headers.
const Algorithm = "AES-256-GCM"

// DEKSize is the size in bytes of a data key.
const DEKSize = keyring.KeySize

// TagSize is the size in bytes of an AES-GCM authentication tag.
const TagSize = 16

var (
	// ErrOpen identifies a sealed data key that failed authentication.
	ErrOpen = errors.New("sealed data key failed authentication")
	// ErrAlgorithm identifies an unsupported wrapping algorithm.
	ErrAlgorithm = errors.New("unsupported wrapping algorithm")
	// ErrDEKSize identifies a data key of the wrong length.
	ErrDEKSize = errors.New("data key has an unexpected size")
	// ErrMalformed identifies sealed key parameters of the wrong length.
	ErrMalformed = errors.New("sealed data key is malformed")
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
	if algorithm == "" || algorithm != Algorithm {
		return nil, fmt.Errorf("%w: %q", ErrAlgorithm, algorithm)
	}

	aead, err := newAEAD(kek)
	if err != nil {
		return nil, err
	}

	nonce, err := newNonce()
	if err != nil {
		return nil, err
	}

	aad := wrappingAAD(version, objectID, algorithm)
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
	if algorithm == "" || algorithm != Algorithm {
		return nil, fmt.Errorf("%w: %q", ErrAlgorithm, algorithm)
	}
	if len(nonce) != nonceSize {
		return nil, fmt.Errorf("%w: nonce is %d bytes, want %d", ErrMalformed, len(nonce), nonceSize)
	}
	if len(ciphertext) != DEKSize+TagSize {
		return nil, fmt.Errorf("%w: ciphertext is %d bytes, want %d", ErrMalformed, len(ciphertext), DEKSize+TagSize)
	}

	aead, err := newAEAD(kek)
	if err != nil {
		return nil, err
	}

	aad := wrappingAAD(version, objectID, algorithm)
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

func wrappingAAD(version uint32, objectID, algorithm string) []byte {
	const domain = "kekring/wrap/v1"

	aad := make([]byte, 0, len(domain)+4+4+len(algorithm)+4+len(objectID))
	aad = append(aad, domain...)
	aad = binary.BigEndian.AppendUint32(aad, version)
	aad = appendLengthPrefixed(aad, algorithm)
	aad = appendLengthPrefixed(aad, objectID)
	return aad
}

func appendLengthPrefixed(dst []byte, value string) []byte {
	dst = binary.BigEndian.AppendUint32(dst, uint32(len(value)))
	return append(dst, value...)
}
