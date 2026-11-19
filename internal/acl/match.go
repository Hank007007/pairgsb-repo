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
		if seg == "**" {
			continue
		}
		if strings.Contains(seg, "**") || strings.ContainsAny(seg, "?[") {
			return ErrInvalidPattern
		}
	}
	return nil
}

// PatternRoot returns the longest literal directory prefix of pattern. It is
// the directory a rule speaks about, with any wildcard and everything below it
// removed.
func PatternRoot(pattern string) string {
	literal := make([]string, 0)
	for _, seg := range pathSegments(Clean(pattern)) {
		if strings.ContainsAny(seg, "*?[") {
			break
		}
		literal = append(literal, seg)
	}
	volume := filepath.VolumeName(pattern)
	if volume != "" {
		// Windows drive root, e.g. C:/a/b -> C:/a/b; no further literals -> C:/.
		if len(literal) == 0 {
			return volume + "/"
		}
		return volume + "/" + strings.Join(literal[1:], "/")
	}
	if len(literal) == 0 {
		return "/"
	}
	return "/" + strings.Join(literal, "/")
}

// matchPath reports whether pattern selects target.
func matchPath(pattern, target string) bool {
	psegs := pathSegments(Clean(pattern))
	tsegs := pathSegments(Clean(target))

	// A pattern ending in a literal segment names both that directory and
	// everything beneath it, equivalent to appending "/**".
	if len(psegs) == 0 {
		psegs = append(psegs, "**")
	} else if last := psegs[len(psegs)-1]; !strings.Contains(last, "*") {
		psegs = append(psegs, "**")
	}

	return matchSegments(psegs, tsegs)
}

// pathSegments splits a cleaned absolute path into its non-empty segments.
func pathSegments(p string) []string {
	volume := filepath.VolumeName(p)
	rest := p[len(volume):]
	segs := make([]string, 0)
	if volume != "" {
		segs = append(segs, strings.TrimSuffix(volume, string(filepath.Separator)))
	}
	for _, seg := range strings.Split(rest, "/") {
		if seg != "" {
			segs = append(segs, seg)
		}
	}
	return segs
}

// segmentCount counts the non-empty segments of a cleaned absolute path.
func segmentCount(p string) int {
	return len(pathSegments(Clean(p)))
}

// matchSegments matches pattern segments against target segments. "*" stands
// for exactly one segment (which may be empty) and "**" for zero or more.
func matchSegments(pattern, target []string) bool {
	for len(pattern) > 0 {
		seg := pattern[0]
		if seg == "**" {
			if len(pattern) == 1 {
				return true
			}
			for skip := 0; skip <= len(target); skip++ {
				if matchSegments(pattern[1:], target[skip:]) {
					return true
				}
			}
			return false
		}
		if len(target) == 0 {
			// A lone "*" may match an empty segment; nothing else can.
			if seg == "*" {
				pattern = pattern[1:]
				continue
			}
			return false
		}
		if !matchSegment(seg, target[0]) {
			return false
		}
		pattern = pattern[1:]
		target = target[1:]
	}
	return len(target) == 0
}

// matchSegment matches a single segment, where "*" is a wildcard for any run
// of characters within that segment. Matching never crosses a slash.
func matchSegment(pattern, name string) bool {
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == name
	}

	parts := strings.Split(pattern, "*")
	if !strings.HasPrefix(name, parts[0]) {
		return false
	}
	if !strings.HasSuffix(name, parts[len(parts)-1]) {
		return false
	}

	rest := name[len(parts[0]) : len(name)-len(parts[len(parts)-1])]
	for _, part := range parts[1 : len(parts)-1] {
		idx := strings.Index(rest, part)
		if idx < 0 {
			return false
		}
		rest = rest[idx+len(part):]
	}
	return true
}
