// Package mac computes and verifies the message authentication codes that
// bind a link's declaration to its issuer's key.
package mac

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"capbaton/internal/token"
)

// ErrMACMismatch is returned when a computed MAC does not match the MAC
// carried by a link.
var ErrMACMismatch = errors.New("mac: signature does not match payload")

// Key is a symmetric signing key shared between a link's issuer and the
// verifier.
type Key []byte

// macInput is the canonical byte string a link's MAC is computed over.
//
// The format is a sequence of length-prefixed fields so that no two
// distinct payloads can produce the same input. PrevMAC is included so
// that a link cannot be lifted out of the chain it was issued into.
func macInput(p token.Payload) []byte {
	var buf []byte
	appendField := func(label, value string) {
		buf = append(buf, label...)
		buf = append(buf, '=')
		buf = append(buf, fmt.Sprintf("%d:", len(value))...)
		buf = append(buf, value...)
		buf = append(buf, '\n')
	}
	appendField("token_id", p.TokenID)
	appendField("issuer", p.Issuer)
	appendField("subject", p.Subject)
	appendField("prev_mac", p.PrevMAC)
	appendField("issued_at", p.IssuedAt.UTC().Format("2006-01-02T15:04:05.000000000Z"))
	for _, s := range p.Scopes {
		appendField("scope", s)
	}
	for _, c := range p.Caveats {
		appendField("caveat", c.Name+"\x00"+c.Value)
	}
	return buf
}

// Compute returns the hex-encoded MAC of p under key.
func Compute(key Key, p token.Payload) string {
	h := hmac.New(sha256.New, key)
	h.Write(macInput(p))
	return hex.EncodeToString(h.Sum(nil))
}

// Verify recomputes the MAC of link.Payload under key and compares it to
// link.MAC in constant time.
func Verify(key Key, link token.Link) error {
	want := Compute(key, link.Payload)
	if !hmac.Equal([]byte(want), []byte(link.MAC)) {
		return ErrMACMismatch
	}
	return nil
}
