package parse

import (
	"fmt"

	"gqladmit/internal/graphql"
)

// ExpandedOperation is an operation together with the selection set that
// results from resolving every fragment spread it contains. The original
// operation is kept for callers that need the request as it was written.
type ExpandedOperation struct {
	Operation    *graphql.Operation
	SelectionSet []graphql.Selection
}

// ExpandedDocument holds one expanded operation per operation in the source
// document. Fragment definitions are not repeated here; every spread has
// already been resolved against them.
type ExpandedDocument struct {
	Operations []*ExpandedOperation
}

// Expand resolves fragment spreads so that each operation carries the full
// selection tree a backend would execute.
func Expand(doc *graphql.Document) (*ExpandedDocument, error) {
	out := &ExpandedDocument{Operations: make([]*ExpandedOperation, 0, len(doc.Operations))}

	for _, op := range doc.Operations {
		visited := make(map[string]bool, len(doc.Fragments))
		sel, err := expandSelections(doc, op.SelectionSet, visited)
		if err != nil {
			return nil, err
		}
		out.Operations = append(out.Operations, &ExpandedOperation{
			Operation:    op,
			SelectionSet: sel,
		})
	}

	return out, nil
}

func expandSelections(doc *graphql.Document, sels []graphql.Selection, visited map[string]bool) ([]graphql.Selection, error) {
	if len(sels) == 0 {
		return nil, nil
	}

	out := make([]graphql.Selection, 0, len(sels))
	for _, sel := range sels {
		switch s := sel.(type) {
		case *graphql.Field:
			sub, err := expandSelections(doc, s.SelectionSet, visited)
			if err != nil {
				return nil, err
			}
			clone := *s
			clone.SelectionSet = sub
			out = append(out, &clone)

		case *graphql.InlineFragment:
			sub, err := expandSelections(doc, s.SelectionSet, visited)
			if err != nil {
				return nil, err
			}
			clone := *s
			clone.SelectionSet = sub
			out = append(out, &clone)

		case *graphql.FragmentSpread:
			frag, ok := doc.Fragments[s.Name]
			if !ok {
				return nil, fmt.Errorf("unknown fragment %q", s.Name)
			}
			if visited[s.Name] {
				continue
			}
			sub, err := expandSelections(doc, frag.SelectionSet, visited)
			if err != nil {
				return nil, err
			}
			visited[s.Name] = true
			out = append(out, sub...)

		default:
			return nil, fmt.Errorf("unsupported selection %T", sel)
		}
	}

	return out, nil
}
