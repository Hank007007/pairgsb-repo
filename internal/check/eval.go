package check

import (
	"tuplegrate/internal/schema"
	"tuplegrate/internal/subject"
	"tuplegrate/internal/tuple"
)

// evalRewrite evaluates a rewrite expression for requested on object.
//
// The relation argument is the relation under whose definition the expression
// is being evaluated; `self` and bare relation references resolve against it.
func (s *evalState) evalRewrite(node schema.Node, object tuple.Object, relation string, requested tuple.Subject, ctx Context) (bool, error) {
	switch n := node.(type) {
	case *schema.Self:
		return s.evalSelf(object, relation, requested, ctx)

	case *schema.RelationRef:
		return s.evalRelation(object, n.Relation, requested, ctx)

	case *schema.TupleToUserset:
		return s.evalTupleToUserset(n, object, relation, requested, ctx)

	case *schema.Union:
		for _, child := range n.Children {
			ok, err := s.evalRewrite(child, object, relation, requested, ctx)
			if err != nil || ok {
				return ok, err
			}
		}
		return false, nil

	case *schema.Intersection:
		for _, child := range n.Children {
			ok, err := s.evalRewrite(child, object, relation, requested, ctx)
			if err != nil || !ok {
				return ok, err
			}
		}
		return true, nil

	case *schema.Difference:
		left, err := s.evalRewrite(n.A, object, relation, requested, ctx)
		if err != nil {
			return false, err
		}
		right, err := s.evalRewrite(n.B, object, relation, requested, ctx)
		if err != nil {
			return false, err
		}
		return left || right, nil

	default:
		return false, newError(CodeUnknownRelation, "unsupported rewrite node %T", node)
	}
}

// evalSelf matches the tuples stored directly on object#relation.
func (s *evalState) evalSelf(object tuple.Object, relation string, requested tuple.Subject, ctx Context) (bool, error) {
	for _, stored := range s.view.tuples.Subjects(object, relation) {
		if subject.Match(stored, requested) {
			return true, nil
		}
		target, targetRelation, expand := subject.Expand(stored)
		if !expand {
			continue
		}
		inner, err := ctx.Descend(descendUserset)
		if err != nil {
			return false, err
		}
		ok, err := s.evalRelation(target, targetRelation, requested, inner)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// evalRelation evaluates relation on object.
func (s *evalState) evalRelation(object tuple.Object, relation string, requested tuple.Subject, ctx Context) (bool, error) {
	node, ok := s.rewriteFor(object, relation)
	if !ok {
		return false, nil
	}
	inner, err := ctx.Descend(descendRelation)
	if err != nil {
		return false, err
	}
	return s.evalRewrite(node, object, relation, requested, inner)
}

// evalTupleToUserset follows the tuples of n.Tupleset to reach other objects,
// then evaluates a relation on each of them.
func (s *evalState) evalTupleToUserset(n *schema.TupleToUserset, object tuple.Object, relation string, requested tuple.Subject, ctx Context) (bool, error) {
	for _, stored := range s.view.tuples.Subjects(object, n.Tupleset) {
		target, ok := stored.Object()
		if !ok {
			continue
		}
		inner, err := ctx.Descend(descendTupleset)
		if err != nil {
			return false, err
		}
		ok, err = s.evalRelation(target, n.Tupleset, requested, inner)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}
