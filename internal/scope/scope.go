// Package scope parses capability scope strings of the form
// "action:resource" and answers containment questions about them.
package scope

import (
	"fmt"
	"strings"
)

// Scope is a single capability: an action performed against a resource.
//
// The textual form is "<action>:<resource>", where the action may itself
// contain colons (for example "s3:read"), and the resource is a
// slash-separated path (for example "bucket/incident-42").
type Scope struct {
	Action   string
	Resource string
}

// String renders the scope back into its textual form.
func (s Scope) String() string {
	return s.Action + ":" + s.Resource
}

// Parse parses a scope string. The action and the resource are separated
// by the final colon in the string, so actions such as "s3:read" keep
// their internal colon.
func Parse(raw string) (Scope, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Scope{}, fmt.Errorf("scope: empty scope string")
	}
	i := strings.LastIndex(raw, ":")
	if i < 0 {
		return Scope{}, fmt.Errorf("scope: %q is missing an action/resource separator", raw)
	}
	action := strings.TrimSpace(raw[:i])
	resource := strings.TrimSpace(raw[i+1:])
	if action == "" {
		return Scope{}, fmt.Errorf("scope: %q has an empty action", raw)
	}
	if resource == "" {
		return Scope{}, fmt.Errorf("scope: %q has an empty resource", raw)
	}
	return Scope{Action: action, Resource: resource}, nil
}

// ParseAll parses a list of scope strings, rejecting duplicates.
func ParseAll(raw []string) ([]Scope, error) {
	out := make([]Scope, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, r := range raw {
		s, err := Parse(r)
		if err != nil {
			return nil, err
		}
		key := s.String()
		if _, dup := seen[key]; dup {
			return nil, fmt.Errorf("scope: duplicate scope %q", key)
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	return out, nil
}

// IsSubset reports whether child is contained in parent, that is whether
// holding parent is enough to exercise child.
//
// The action must match exactly. The resource is compared segment by
// segment: a child resource is contained in a parent resource when the
// parent's segments are a leading run of the child's segments. A
// resource is contained in itself.
func IsSubset(child, parent Scope) bool {
	if child.Action != parent.Action {
		return false
	}
	parentSegments := strings.Split(parent.Resource, "/")
	childSegments := strings.Split(child.Resource, "/")
	if len(childSegments) < len(parentSegments) {
		return false
	}
	for i, segment := range parentSegments {
		if childSegments[i] != segment {
			return false
		}
	}
	return true
}

// IsSetSubset reports whether every scope in child is contained in at
// least one scope in parent.
func IsSetSubset(child, parent []Scope) bool {
	for _, c := range child {
		covered := false
		for _, p := range parent {
			if IsSubset(c, p) {
				covered = true
				break
			}
		}
		if !covered {
			return false
		}
	}
	return true
}

// Missing returns the scopes in child that are not contained in any scope
// in parent, in the order they appear in child.
func Missing(child, parent []Scope) []Scope {
	var out []Scope
	for _, c := range child {
		covered := false
		for _, p := range parent {
			if IsSubset(c, p) {
				covered = true
				break
			}
		}
		if !covered {
			out = append(out, c)
		}
	}
	return out
}
