package seal_test

import (
	"bytes"
	"testing"

	"sealrelay/internal/seal"
)

func TestNoncesAreUnique(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, 32)
	seen := make(map[string]struct{})
	for i := 0; i < 64; i++ {
		nonce, _, err := seal.Encrypt(key, []byte("payload"))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(nonce, make([]byte, seal.NonceSize)) {
			t.Fatal("nonce is all zero")
		}
		if _, ok := seen[string(nonce)]; ok {
			t.Fatalf("nonce reused: %x", nonce)
		}
		seen[string(nonce)] = struct{}{}
	}
}
