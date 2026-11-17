// Package cost measures the work a query asks of the backend: how many fields
// it selects and how deeply they nest.
package cost

import (
	"gqladmit/internal/graphql"
	"gqladmit/internal/parse"
)

// Count returns the number of fields the operation selects. Every field node
// counts once; a field reached through several aliases counts once per alias.
func Count(op *parse.ExpandedOperation) int {
	return countSelections(op.SelectionSet)
}

func countSelections(sels []graphql.Selection) int {
	total := 0
	for _, sel := range sels {
		switch s := sel.(type) {
		case *graphql.Field:
			total++
			total += countSelections(s.SelectionSet)
		case *graphql.InlineFragment:
			total += countSelections(s.SelectionSet)
		case *graphql.FragmentSpread:
			total++
		}
	}
	return total
}

// Depth returns the maximum field nesting level of the operation. A leaf
// field has depth 1 and a field counts one level more than the deepest field
// beneath it.
func Depth(op *parse.ExpandedOperation) int {
	return depthOf(op.SelectionSet)
}

func depthOf(sels []graphql.Selection) int {
	max := 0
	for _, sel := range sels {
		var d int
		switch s := sel.(type) {
		case *graphql.Field:
			d = 1 + depthOf(s.SelectionSet)
		case *graphql.InlineFragment:
			d = depthOf(s.SelectionSet)
		case *graphql.FragmentSpread:
			d = 1
		}
		if d > max {
			max = d
		}
	}
	return max
}
