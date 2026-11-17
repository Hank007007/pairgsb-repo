package control

import (
	"crypto/sha256"

	"sealrelay/internal/bundle"
)

// Secret is one named credential carried by a bundle.
type Secret = bundle.Secret

func digestOf(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}
