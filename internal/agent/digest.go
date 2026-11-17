package agent

import "crypto/sha256"

func digestOf(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}
