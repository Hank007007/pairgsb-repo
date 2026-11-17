// Package match implements the individual comparison rules used by the
// evaluator: user and host matching, executable matching, and argument
// matching.
package match

import "path/filepath"

// Command reports whether the executable path given by argv0 is the command
// named by pattern.
func Command(pattern, argv0 string) bool {
	if pattern == "" || argv0 == "" {
		return false
	}
	return filepath.Base(pattern) == filepath.Base(argv0)
}
