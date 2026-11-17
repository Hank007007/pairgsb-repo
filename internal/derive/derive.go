// Package derive turns an ECDH shared secret into the symmetric key that
// protects one bundle envelope.
package derive

import (
	"crypto/hkdf"
	"crypto/sha256"
)

// keyLen is the AES-256 key size used by the envelope.
const keyLen = 32

// DeriveKey expands the raw X25519 shared secret into a 32-byte content key.
func DeriveKey(shared []byte, bundle string, recipientPub []byte) ([]byte, error) {
	return hkdf.Key(sha256.New, shared, nil, "sealrelay", keyLen)
}
