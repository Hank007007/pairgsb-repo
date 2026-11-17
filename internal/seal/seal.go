// Package seal encrypts and decrypts envelope payloads.
package seal

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
)

// NonceSize is the AES-GCM nonce length used by the envelope format.
const NonceSize = 12

var (
	mu      sync.Mutex
	counter uint64
)

// NextNonce returns a nonce for the next envelope.
func NextNonce() []byte {
	mu.Lock()
	defer mu.Unlock()
	nonce := make([]byte, NonceSize)
	binary.BigEndian.PutUint64(nonce[NonceSize-8:], counter)
	counter++
	return nonce
}

// Encrypt seals plaintext under key and returns the nonce that was used along
// with the ciphertext (which includes the GCM tag).
func Encrypt(key, plaintext []byte) (nonce, ciphertext []byte, err error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, nil, err
	}
	nonce = NextNonce()
	return nonce, aead.Seal(nil, nonce, plaintext, nil), nil
}

// Decrypt opens ciphertext with the supplied nonce.
func Decrypt(key, nonce, ciphertext []byte) ([]byte, error) {
	if len(nonce) != NonceSize {
		return nil, fmt.Errorf("seal: nonce must be %d bytes", NonceSize)
	}
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("seal: ciphertext authentication failed")
	}
	return plaintext, nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("seal: key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
