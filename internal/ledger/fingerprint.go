package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"findingsledger/internal/source"
)

// FingerprintInput is everything the fingerprint algorithm is allowed to look
// at.
type FingerprintInput struct {
	RuleID  string
	File    string
	Line    int // retained for scanner input compatibility; not part of identity
	Snippet string
}

// Compute derives the stable identity of a finding.
func Compute(in FingerprintInput) string {
	key := source.Normalize(in.File) + "\x1f" + in.RuleID + "\x1f" + normalizeSnippet(in.Snippet)
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func normalizeSnippet(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.Join(strings.Fields(s), " ")
	return foldASCII(s)
}

func foldASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
