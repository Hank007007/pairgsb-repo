package schema

import (
	"fmt"
	"sort"
	"strings"
)

// Walk visits every node of the rewrite tree in depth-first order. It returns
// the first non-nil error produced by fn.
func Walk(node Node, fn func(Node) error) error {
	if node == nil {
		return nil
	}
	if err := fn(node); err != nil {
		return err
	}
	switch n := node.(type) {
	case *Union:
		for _, child := range n.Children {
			if err := Walk(child, fn); err != nil {
				return err
			}
		}
	case *Intersection:
		for _, child := range n.Children {
			if err := Walk(child, fn); err != nil {
				return err
			}
		}
	case *Difference:
		if err := Walk(n.A, fn); err != nil {
			return err
		}
		return Walk(n.B, fn)
	}
	return nil
}

// Format renders a rewrite tree in the canonical schema syntax.
func Format(node Node) string {
	switch n := node.(type) {
	case nil:
		return ""
	case *Self:
		return "self"
	case *RelationRef:
		return n.Relation
	case *TupleToUserset:
		return n.ComputedUserset + " from " + n.Tupleset
	case *Union:
		parts := make([]string, 0, len(n.Children))
		for _, child := range n.Children {
			parts = append(parts, Format(child))
		}
		return strings.Join(parts, " | ")
	case *Intersection:
		parts := make([]string, 0, len(n.Children))
		for _, child := range n.Children {
			parts = append(parts, Format(child))
		}
		return strings.Join(parts, " & ")
	case *Difference:
		return Format(n.A) + " but not " + Format(n.B)
	default:
		return fmt.Sprintf("<unknown %T>", node)
	}
}

// References collects the relation names mentioned by computed usersets and
// tuple-to-userset references in the tree, sorted and de-duplicated.
func References(node Node) []string {
	seen := make(map[string]struct{})
	_ = Walk(node, func(n Node) error {
		switch ref := n.(type) {
		case *RelationRef:
			seen[ref.Relation] = struct{}{}
		case *TupleToUserset:
			seen[ref.ComputedUserset] = struct{}{}
			seen[ref.Tupleset] = struct{}{}
		}
		return nil
	})
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
