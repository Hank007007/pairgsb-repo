// Package tuple provides parsing, versioned storage and indexing for the
// relationship tuples that tuplegrate evaluates against.
//
// A tuple has the form object#relation@subject. The store keeps a change log
// so that any revision that has not been garbage collected can be
// materialised as an immutable view.
package tuple

import (
	"fmt"
	"strings"
)

// Object identifies a namespaced entity, written namespace:id.
type Object struct {
	Namespace string
	ID        string
}

// String renders the object in namespace:id form.
func (o Object) String() string {
	return o.Namespace + ":" + o.ID
}

// IsZero reports whether the object is the zero value.
func (o Object) IsZero() bool {
	return o.Namespace == "" && o.ID == ""
}

// ParseObject parses a namespace:id reference.
func ParseObject(s string) (Object, error) {
	namespace, id, err := splitName(s)
	if err != nil {
		return Object{}, err
	}
	return Object{Namespace: namespace, ID: id}, nil
}

func splitName(s string) (namespace string, id string, err error) {
	idx := strings.IndexByte(s, ':')
	if idx < 0 {
		return "", "", fmt.Errorf("tuple: %q is missing a namespace separator", s)
	}
	namespace = s[:idx]
	id = s[idx+1:]
	if !validNamespace(namespace) {
		return "", "", fmt.Errorf("tuple: %q has an invalid namespace", s)
	}
	if id == "" {
		return "", "", fmt.Errorf("tuple: %q has an empty identifier", s)
	}
	if strings.ContainsAny(id, "#@") {
		return "", "", fmt.Errorf("tuple: %q has an invalid identifier", s)
	}
	return namespace, id, nil
}

func validNamespace(ns string) bool {
	if ns == "" {
		return false
	}
	for i := 0; i < len(ns); i++ {
		c := ns[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c == '_':
		case c >= '0' && c <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func validRelationName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c == '_', c == '-':
		case c >= '0' && c <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
