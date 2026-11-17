// Package match implements the individual comparison rules used by the
// evaluator: user and host matching, executable matching, and argument
// matching.
package match

import (
	"os"
	"path/filepath"
)

// Command reports whether the executable path given by argv0 is the command
// named by pattern.
func Command(pattern, argv0 string) bool {
	if pattern == "" || argv0 == "" {
		return false
	}

	if normalizePath(pattern) == normalizePath(argv0) {
		return true
	}

	patternInfo, patternErr := os.Stat(pattern)
	argv0Info, argv0Err := os.Stat(argv0)
	if patternErr == nil && argv0Err == nil {
		return os.SameFile(patternInfo, argv0Info)
	}
	return false
}

// normalizePath returns an absolute, lexically cleaned path so that different
// spellings of the same location compare equal.
func normalizePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return abs
}
