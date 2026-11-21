// Package source contains helpers for describing where a finding was seen.
package source

import (
	"path/filepath"
	"strings"
)

// Normalize turns a scanner-supplied path into the canonical form used when a
// finding fingerprint is computed.
func Normalize(p string) string {
	if strings.TrimSpace(p) == "" {
		return ""
	}
	return filepath.Clean(p)
}
