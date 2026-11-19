package tuple

import (
	"fmt"
	"strings"
)

// Subject is the right-hand side of a tuple. It is one of:
//
//   - a terminal subject, namespace:id (Relation is empty);
//   - a subject set, namespace:id#relation;
//   - a wildcard subject, namespace:* (Relation is empty).
type Subject struct {
	Namespace string
	ID        string
	Relation  string
}

// String renders the subject in its canonical form.
func (s Subject) String() string {
	if s.Relation == "" {
		return s.Namespace + ":" + s.ID
	}
	return s.Namespace + ":" + s.ID + "#" + s.Relation
}

// IsTerminal reports whether the subject names a single entity rather than a
// subject set.
func (s Subject) IsTerminal() bool {
	return s.Relation == ""
}

// IsUserset reports whether the subject is a subject set.
func (s Subject) IsUserset() bool {
	return s.Relation != ""
}

// IsWildcard reports whether the subject is a namespace wildcard.
func (s Subject) IsWildcard() bool {
	return s.Relation == "" && s.ID == "*"
}

// Object returns the object referenced by the subject. Wildcard subjects do
// not reference a single object, so the second result is false for them.
func (s Subject) Object() (Object, bool) {
	if s.Namespace == "" || s.ID == "" || s.IsWildcard() {
		return Object{}, false
	}
	return Object{Namespace: s.Namespace, ID: s.ID}, true
}

// ParseSubject parses a terminal subject, subject set or wildcard subject.
func ParseSubject(s string) (Subject, error) {
	if s == "" {
		return Subject{}, fmt.Errorf("tuple: empty subject")
	}

	name := s
	relation := ""
	hasRelation := false
	if idx := strings.IndexByte(s, '#'); idx >= 0 {
		name = s[:idx]
		relation = s[idx+1:]
		hasRelation = true
		if relation == "" {
			return Subject{}, fmt.Errorf("tuple: subject %q has an empty relation", s)
		}
		if !validRelationName(relation) {
			return Subject{}, fmt.Errorf("tuple: subject %q has an invalid relation", s)
		}
		if strings.ContainsRune(relation, '#') {
			return Subject{}, fmt.Errorf("tuple: subject %q has more than one relation", s)
		}
	}

	namespace, id, err := splitName(name)
	if err != nil {
		return Subject{}, err
	}

	if id == "*" {
		if hasRelation {
			return Subject{}, fmt.Errorf("tuple: wildcard subject %q cannot carry a relation", s)
		}
		return Subject{Namespace: namespace, ID: "*"}, nil
	}

	return Subject{Namespace: namespace, ID: id, Relation: relation}, nil
}
