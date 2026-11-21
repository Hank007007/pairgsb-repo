package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"

	"findingsledger/internal/source"
)

// FingerprintInput is everything the fingerprint algorithm is allowed to look
// at.
type FingerprintInput struct {
	RuleID  string
	File    string
	Line    int
	Snippet string
}

// Compute derives the stable identity of a finding.
func Compute(in FingerprintInput) string {
	key := in.RuleID + "|" + source.Normalize(in.File) + "|" + strconv.Itoa(in.Line)
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
