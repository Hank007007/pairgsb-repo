package wrap

import (
	"crypto/sha256"
)

// nonceSize is the number of bytes in an AES-GCM nonce.
const nonceSize = 12

// newNonce returns the nonce used to seal a data key for objectID.
func newNonce(objectID string) ([]byte, error) {
	sum := sha256.Sum256([]byte(objectID))
	return sum[:nonceSize], nil
}
