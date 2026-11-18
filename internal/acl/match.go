package acl

import (
	"errors"
	"path"
	"path/filepath"
	"strings"
)

// Errors returned while interpreting a request path or a pattern.
var (
	ErrInvalidPath    = errors.New("acl: path must be absolute")
	ErrInvalidPattern = errors.New("acl: invalid path pattern")
)

// Clean puts a path into the form used for matching: slash separated, without
// repeated separators, dot segments, or a trailing slash.
func Clean(p string) string {
	return path.Clean(filepath.ToSlash(p))
}

// NormalizePath validates and normalises a request path.
func NormalizePath(p string) (string, error) {
	if p == "" || strings.ContainsRune(p, 0) {
		return "", ErrInvalidPath
	}
	if !path.IsAbs(p) && !filepath.IsAbs(p) {
		return "", ErrInvalidPath
	}
	return Clean(p), nil
}

// ValidPattern reports whether pattern is an acceptable configuration entry.
func ValidPattern(pattern string) error {
	if pattern == "" || strings.ContainsRune(pattern, 0) {
		return ErrInvalidPattern
	}
	if !path.IsAbs(pattern) && !filepath.IsAbs(pattern) {
		return ErrInvalidPattern
	}
	for _, seg := range strings.Split(filepath.ToSlash(pattern), "/") {
		if seg == "." || seg == ".." {
			return ErrInvalidPattern
		}
	}
	return nil
}

// PatternRoot returns the longest literal directory prefix of pattern. It is
// the directory a rule speaks about, with any wildcard and everything below it
// removed.
func PatternRoot(pattern string) string {
	p := Clean(pattern)
	p = strings.TrimSuffix(p, "/**")
	p = strings.TrimSuffix(p, "/*")

	segs := strings.Split(p, "/")
	literal := make([]string, 0, len(segs))
	for _, seg := range segs {
		if strings.ContainsAny(seg, "*?[") {
			break
		}
		literal = append(literal, seg)
	}
	root := strings.Join(literal, "/")
	if root == "" {
		return "/"
	}
	return root
}

// matchPath reports whether pattern selects target.
func matchPath(pattern, target string) bool {
	p := Clean(pattern)
	t := Clean(target)

	p = strings.TrimSuffix(p, "/**")

	if i := strings.IndexByte(p, '*'); i >= 0 {
		parts := strings.Split(p, "*")
		if !strings.HasPrefix(t, parts[0]) {
			return false
		}
		return strings.HasSuffix(t, parts[len(parts)-1])
	}

	return strings.HasPrefix(t, p)
}
