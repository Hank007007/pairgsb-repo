// Package source contains helpers for describing where a finding was seen.
package source

import (
	"path"
	"strings"
)

// Normalize turns a scanner-supplied path into the canonical form used when a
// finding fingerprint is computed.
func Normalize(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = strings.ReplaceAll(p, `\`, "/")
	p = path.Clean(p)
	return foldASCII(p)
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
