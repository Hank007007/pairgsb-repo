package policy

import "strings"

// splitSegments breaks a resource path or pattern into its non-empty segments.
func splitSegments(p string) []string {
	parts := strings.Split(p, "/")
	segments := parts[:0]
	for _, part := range parts {
		if part != "" {
			segments = append(segments, part)
		}
	}
	return segments
}

// NormalizePath collapses repeated slashes and strips a trailing slash.
func NormalizePath(p string) string {
	segments := splitSegments(p)
	if len(segments) == 0 {
		return "/"
	}
	return "/" + strings.Join(segments, "/")
}

// MatchResource reports whether pattern applies to path.
func MatchResource(pattern, path string) bool {
	if pattern == "" {
		return false
	}
	path = NormalizePath(path)
	if !strings.Contains(pattern, "*") {
		return strings.HasPrefix(path, pattern)
	}
	return matchSegments(splitSegments(pattern), splitSegments(path))
}

// matchSegments walks a pattern's segments against a path's segments.
func matchSegments(pattern, path []string) bool {
	for i, segment := range pattern {
		if segment == "*" || segment == "**" {
			rest := pattern[i+1:]
			for cut := 0; cut <= len(path); cut++ {
				if matchSegments(rest, path[cut:]) {
					return true
				}
			}
			return false
		}
		if len(path) == 0 {
			return false
		}
		if !strings.HasPrefix(path[0], segment) {
			return false
		}
		path = path[1:]
	}
	return true
}
