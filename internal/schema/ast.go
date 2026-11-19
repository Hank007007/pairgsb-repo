// Package schema parses relation schemas into an immutable rewrite tree.
//
// The grammar is documented in docs/relation-model.md §2. A schema is a list
// of type declarations, each containing relation definitions whose
// right-hand sides are boolean rewrite expressions over self, computed
// usersets and tuple-to-userset references.
package schema

import (
	"fmt"
	"sort"
	"strings"
)

// Node is a node in a compiled rewrite expression.
type Node interface {
	rewrite()
}

// Self matches a tuple written directly on the current object and relation.
type Self struct{}

func (*Self) rewrite() {}

// RelationRef evaluates another relation on the same object.
type RelationRef struct {
	Relation string
}

func (*RelationRef) rewrite() {}

// TupleToUserset follows the tuples of Tupleset to other objects and
// evaluates ComputedUserset there.
type TupleToUserset struct {
	ComputedUserset string
	Tupleset        string
}

func (*TupleToUserset) rewrite() {}

// Union is the logical OR of its children.
type Union struct {
	Children []Node
}

func (*Union) rewrite() {}

// Intersection is the logical AND of its children.
type Intersection struct {
	Children []Node
}

func (*Intersection) rewrite() {}

// Difference is A minus B.
type Difference struct {
	A Node
	B Node
}

func (*Difference) rewrite() {}

// Relation is one compiled relation definition.
type Relation struct {
	Name     string
	Rewrite  Node
	Raw      string
	Position int
}

// Type is a compiled namespace declaration.
type Type struct {
	Name      string
	Relations map[string]*Relation
	Order     []string
}

// Relation returns the named relation, or nil if the type does not define it.
func (t *Type) Relation(name string) *Relation {
	if t == nil {
		return nil
	}
	return t.Relations[name]
}

// Names lists the type's relation names in declaration order.
func (t *Type) Names() []string {
	if t == nil {
		return nil
	}
	return append([]string(nil), t.Order...)
}

// Schema is a compiled set of type declarations.
type Schema struct {
	types map[string]*Type
	order []string
}

// Type returns the named namespace, or nil if it is not declared.
func (s *Schema) Type(name string) *Type {
	if s == nil {
		return nil
	}
	return s.types[name]
}

// Names lists the declared type names in declaration order.
func (s *Schema) Names() []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.order...)
}

// Relation looks up a relation within a namespace.
func (s *Schema) Relation(namespace, relation string) (*Relation, bool) {
	t := s.Type(namespace)
	if t == nil {
		return nil, false
	}
	rel := t.Relation(relation)
	if rel == nil {
		return nil, false
	}
	return rel, true
}

// String renders the schema in a canonical, sorted form. It is useful for
// diagnostics and tests.
func (s *Schema) String() string {
	var b strings.Builder
	names := s.Names()
	sort.Strings(names)
	for _, name := range names {
		t := s.types[name]
		b.WriteString("type ")
		b.WriteString(t.Name)
		b.WriteString(" {\n")
		for _, relName := range t.Order {
			rel := t.Relations[relName]
			fmt.Fprintf(&b, "    relation %s = %s\n", rel.Name, rel.Raw)
		}
		b.WriteString("}\n")
	}
	return b.String()
}
