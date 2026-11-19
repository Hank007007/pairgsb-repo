package check

import (
	"tuplegrate/internal/schema"
	"tuplegrate/internal/tuple"
)

// MaxDepth is the descent budget granted to a single evaluation path.
const MaxDepth = 25

// descentMode describes the kind of nested evaluation being performed.
type descentMode uint8

const (
	// descendUserset expands a subject set stored in a tuple.
	descendUserset descentMode = iota

	// descendRelation evaluates a computed userset on the same object.
	descendRelation

	// descendTupleset follows a tuple-to-userset reference to another
	// object.
	descendTupleset
)

// Context carries the budget available to the current evaluation path.
// Budgets are copied, never shared, so sibling operands start from the same
// remaining depth.
type Context struct {
	remaining int
}

// newContext returns a context with a full budget.
func newContext() Context {
	return Context{remaining: MaxDepth}
}

// Remaining reports the number of descents still available.
func (c Context) Remaining() int {
	return c.remaining
}

// Descend returns the context to use for one step of nested evaluation.
//
// Expanding a subject set pulls in the tuples of the relation it names, so it
// is charged against the budget.
func (c Context) Descend(mode descentMode) (Context, error) {
	if mode != descendUserset {
		return c, nil
	}
	if c.remaining <= 0 {
		return c, newError(CodeDepthExceeded, "descent budget of %d exhausted", MaxDepth)
	}
	c.remaining--
	return c, nil
}

// evalState bundles the immutable inputs of one check.
type evalState struct {
	view *View
}

// rewriteFor returns the rewrite expression for relation on object.
func (s *evalState) rewriteFor(object tuple.Object, relation string) (schema.Node, bool) {
	rel, ok := s.view.schema.Relation(object.Namespace, relation)
	if !ok {
		return nil, false
	}
	return rel.Rewrite, true
}
