// Package derive turns an ECDH shared secret into the symmetric key that
// protects one bundle envelope.
package derive

import (
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/binary"
)

// keyLen is the AES-256 key size used by the envelope.
const keyLen = 32

// DeriveKey expands the raw X25519 shared secret into a 32-byte content key.
func DeriveKey(shared []byte, bundle string, recipientPub []byte) ([]byte, error) {
	info := make([]byte, 0, len("sealrelay/v1")+8+len(bundle)+len(recipientPub))
	info = append(info, "sealrelay/v1"...)
	info = appendLengthPrefixed(info, []byte(bundle))
	info = appendLengthPrefixed(info, recipientPub)
	return hkdf.Key(sha256.New, shared, nil, string(info), keyLen)
}

func appendLengthPrefixed(dst, data []byte) []byte {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(data)))
	dst = append(dst, length[:]...)
	return append(dst, data...)
}
