// Package cache stores authorization decisions keyed by request dimensions.
package cache

import (
	"fmt"
	"strings"
)

// Key identifies one cached decision. It is built from the request
// dimensions that the design in docs/pdp-cache.md requires.
type Key string

// BuildKey derives the cache key for one request under a policy revision.
//
// tenant, subject, action and resource are request fields. attrs is the
// caller-supplied attribute set, which is unordered.
func BuildKey(tenant, subject, action, resource string, attrs map[string]string, revision string) Key {
	var b strings.Builder
	b.WriteString(subject)
	b.WriteString("|")
	b.WriteString(resource)
	b.WriteString("|")
	b.WriteString(fingerprint(attrs))
	b.WriteString("|")
	b.WriteString(revision)
	return Key(b.String())
}

// fingerprint condenses an attribute map to a short string.
func fingerprint(attrs map[string]string) string {
	if len(attrs) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(attrs))
	for name, value := range attrs {
		parts = append(parts, fmt.Sprintf("%s=%s", name, value))
	}
	return strings.Join(parts, ";")
}
