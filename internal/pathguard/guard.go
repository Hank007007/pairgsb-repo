// Package pathguard maps archive entry names onto locations inside a job root.
package pathguard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pairgsb/unpackfence/internal/fault"
)

// CheckName validates an entry name before it is joined with a root.
//
// Names are interpreted independently of the host platform: both slash forms
// are separators, and Windows drive and UNC prefixes are treated as absolute
// everywhere.
func CheckName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: empty name", fault.ErrUnsafeName)
	}
	if strings.IndexByte(name, 0) >= 0 {
		return fmt.Errorf("%w: %q contains a NUL byte", fault.ErrUnsafeName, name)
	}

	normalized := normalizeSeparators(name)
	switch {
	case strings.HasPrefix(normalized, "//"):
		return fmt.Errorf("%w: %q is a UNC path", fault.ErrUnsafeName, name)
	case strings.HasPrefix(normalized, "/"):
		return fmt.Errorf("%w: %q is absolute", fault.ErrUnsafeName, name)
	case hasDrivePrefix(normalized):
		return fmt.Errorf("%w: %q is a drive-absolute path", fault.ErrUnsafeName, name)
	}

	if normalized == "." {
		return fmt.Errorf("%w: empty name", fault.ErrUnsafeName)
	}
	return nil
}

// Resolve joins name onto root and reports the location the entry would use.
func Resolve(root, name string) (string, error) {
	if err := CheckName(name); err != nil {
		return "", err
	}

	root = filepath.Clean(root)
	target := filepath.Join(root, filepath.FromSlash(normalizeSeparators(name)))
	if !Within(root, target) {
		return "", fmt.Errorf("%w: %q", fault.ErrOutsideRoot, name)
	}
	return target, nil
}

// Within reports whether target is root itself or lies below it. The check is
// made on path segments, never on a string prefix.
func Within(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)

	rel, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel == "." ||
		(rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// ResolveExisting resolves any symlinks in the existing prefix of target and
// returns the concrete path the entry should use. Every symlink encountered
// must resolve to a location under root.
func ResolveExisting(root, target string) (string, error) {
	root = filepath.Clean(root)
	target = filepath.Clean(target)

	probe := target
	var rest []string
	for {
		if _, err := os.Lstat(probe); err == nil {
			resolved, err := filepath.EvalSymlinks(probe)
			if err != nil {
				return "", fmt.Errorf("%w: %q: %v", fault.ErrSymlinkEscape, target, err)
			}
			if !Within(root, resolved) {
				return "", fmt.Errorf("%w: %q resolves outside the job root", fault.ErrSymlinkEscape, target)
			}

			parts := make([]string, 0, 1+len(rest))
			parts = append(parts, resolved)
			parts = append(parts, rest...)
			final := filepath.Join(parts...)
			if !Within(root, final) {
				return "", fmt.Errorf("%w: %q resolves outside the job root", fault.ErrSymlinkEscape, target)
			}
			return final, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}

		parent := filepath.Dir(probe)
		if parent == probe {
			return "", fmt.Errorf("%w: %q has no existing ancestor", fault.ErrSymlinkEscape, target)
		}
		rest = append([]string{filepath.Base(probe)}, rest...)
		probe = parent
	}
}

// CheckSymlinkTarget verifies that a symlink recorded at linkPath and pointing
// at target cannot resolve outside root.
func CheckSymlinkTarget(root, linkPath, target string) error {
	if target == "" {
		return fmt.Errorf("%w: empty target", fault.ErrSymlinkEscape)
	}
	if strings.IndexByte(target, 0) >= 0 {
		return fmt.Errorf("%w: target contains a NUL byte", fault.ErrSymlinkEscape)
	}

	normalized := normalizeSeparators(target)
	var candidate string
	switch {
	case strings.HasPrefix(normalized, "//"):
		return fmt.Errorf("%w: target %q is a UNC path", fault.ErrSymlinkEscape, target)
	case strings.HasPrefix(normalized, "/") || hasDrivePrefix(normalized):
		candidate = filepath.FromSlash(normalized)
	default:
		candidate = filepath.Join(filepath.Dir(linkPath), filepath.FromSlash(normalized))
	}
	candidate = filepath.Clean(candidate)

	if !Within(root, candidate) {
		return fmt.Errorf("%w: target %q resolves outside the job root", fault.ErrSymlinkEscape, target)
	}
	if _, err := ResolveExisting(root, candidate); err != nil {
		return err
	}
	return nil
}

func normalizeSeparators(name string) string {
	return strings.ReplaceAll(name, "\\", "/")
}

func hasDrivePrefix(name string) bool {
	if len(name) < 2 || name[1] != ':' {
		return false
	}
	c := name[0]
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
