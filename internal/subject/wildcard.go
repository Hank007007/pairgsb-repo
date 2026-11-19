// Package subject decides whether a subject stored in a tuple covers the
// subject named by a check request.
package subject

import (
	"tuplegrate/internal/tuple"
)

// Match reports whether the stored subject covers the requested subject.
func Match(stored, requested tuple.Subject) bool {
	if stored == requested {
		return true
	}
	if stored.IsWildcard() {
		return matchesWildcard(stored, requested)
	}
	return false
}

// matchesWildcard handles the case where the stored subject is a namespace
// wildcard.
func matchesWildcard(wildcard, requested tuple.Subject) bool {
	if requested.IsUserset() {
		return true
	}
	return wildcard.Namespace == requested.Namespace
}

// Expand reports whether deciding coverage of a stored subject requires
// evaluating a subject set at the given object and relation. Terminal and
// wildcard subjects never need expansion.
func Expand(stored tuple.Subject) (tuple.Object, string, bool) {
	if !stored.IsUserset() {
		return tuple.Object{}, "", false
	}
	object, ok := stored.Object()
	if !ok {
		return tuple.Object{}, "", false
	}
	return object, stored.Relation, true
}

// NamespaceOf returns the namespace of a subject.
func NamespaceOf(s tuple.Subject) string {
	return s.Namespace
}
