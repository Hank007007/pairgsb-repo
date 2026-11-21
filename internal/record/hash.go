package record

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

// HashInputDomain separates journal hashes from any other use of SHA-256.
const HashInputDomain = "maskseal-journal/v1"

// GenesisHash is the prev_hash value of the first record in a journal.
const GenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

// Hash returns the hex-encoded digest that links r into the journal chain.
func Hash(r Record) string {
	h := sha256.New()
	io.WriteString(h, HashInputDomain)
	h.Write([]byte{0})
	io.WriteString(h, fmt.Sprintf("seq=%d\n", r.Seq))
	io.WriteString(h, "actor="+r.Actor+"\n")
	io.WriteString(h, "action="+r.Action+"\n")
	io.WriteString(h, "ts="+FormatTime(r.Time)+"\n")
	io.WriteString(h, "prev="+r.PrevHash+"\n")

	leaves := Flatten(r.Fields)
	sort.Slice(leaves, func(i, j int) bool {
		return leaves[i].Path < leaves[j].Path
	})
	for _, leaf := range leaves {
		canonical, err := CanonicalValue(leaf.Value)
		if err != nil {
			continue
		}
		io.WriteString(h, "field="+leaf.Path+"="+canonical+"\n")
	}
	return hex.EncodeToString(h.Sum(nil))
}

func formatFloat(f float64) string {
	if !math.IsNaN(f) && !math.IsInf(f, 0) && f == math.Trunc(f) {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// Commit derives the commitment for value at the given field path under salt.
func Commit(salt []byte, path string, value any) (string, error) {
	var b strings.Builder
	if err := writeValue(&b, value); err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte(path))
	mac.Write([]byte{0})
	mac.Write([]byte(b.String()))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// NewSalt returns a fresh secret used for one commitment.
func NewSalt() ([]byte, error) {
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// ErrSeal reports a value that could not be sealed or opened.
var ErrSeal = errors.New("record: seal failure")

// Seal encrypts plaintext under salt, binding it to the salt id and path.
func Seal(salt []byte, saltID, path string, plaintext []byte) ([]byte, error) {
	key := sha256.Sum256(append(append([]byte("maskseal-seal/v1"), salt...), []byte(path)...))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSeal, err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSeal, err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSeal, err)
	}
	aad := []byte(saltID + "\x00" + path)
	return gcm.Seal(nonce, nonce, plaintext, aad), nil
}

// Open decrypts a value produced by Seal.
func Open(salt []byte, saltID, path string, sealed []byte) ([]byte, error) {
	key := sha256.Sum256(append(append([]byte("maskseal-seal/v1"), salt...), []byte(path)...))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSeal, err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSeal, err)
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, fmt.Errorf("%w: short ciphertext", ErrSeal)
	}
	nonce, body := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	aad := []byte(saltID + "\x00" + path)
	out, err := gcm.Open(nil, nonce, body, aad)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSeal, err)
	}
	return out, nil
}
