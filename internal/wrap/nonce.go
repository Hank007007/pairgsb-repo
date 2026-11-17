package wrap

import (
	"crypto/rand"
	"fmt"
)

// nonceSize is the number of bytes in an AES-GCM nonce.
const nonceSize = 12

// newNonce returns a fresh random nonce for sealing a data key.
func newNonce() ([]byte, error) {
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate data key nonce: %w", err)
	}
	return nonce, nil
}
