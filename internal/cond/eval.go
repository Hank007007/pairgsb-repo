package cond

import (
	"fmt"
	"strings"
)

// Result is the three-valued outcome of evaluating a condition.
type Result int

const (
	// False means the expression is decisively not satisfied.
	False Result = iota
	// True means the expression is decisively satisfied.
	True
	// Indeterminate means the expression could not be evaluated.
	Indeterminate
)

// String returns the name of the result.
func (r Result) String() string {
	switch r {
	case True:
		return "true"
	case False:
		return "false"
	default:
		return "indeterminate"
	}
}

// Env supplies attribute values to the evaluator. Get returns nil when the
// path does not exist. Type reports the declared type of an attribute path.
type Env interface {
	Get(path string) any
	Type(path string) (string, bool)
}

// Eval evaluates a condition tree against env.
func Eval(n *Node, env Env) Result {
	if n == nil {
		return True
	}
	switch {
	case n.And != nil:
		return evalAnd(n.And, env)
	case n.Or != nil:
		return evalOr(n.Or, env)
	case n.Not != nil:
		return evalNot(*n.Not, env)
	case n.Cmp != nil:
		return evalCmp(*n.Cmp, env)
	case n.In != nil:
		return evalIn(*n.In, env)
	case n.Contains != nil:
		return evalContains(*n.Contains, env)
	case n.Present != nil:
		return evalPresent(*n.Present, env)
	}
	return Indeterminate
}

func evalAnd(nodes []Node, env Env) Result {
	result := True
	for i := range nodes {
		switch Eval(&nodes[i], env) {
		case False:
			return False
		case Indeterminate:
			result = Indeterminate
		}
	}
	return result
}

func evalOr(nodes []Node, env Env) Result {
	result := False
	for i := range nodes {
		switch Eval(&nodes[i], env) {
		case True:
			return True
		case Indeterminate:
			result = Indeterminate
		}
	}
	return result
}

func evalNot(n Node, env Env) Result {
	switch Eval(&n, env) {
	case True:
		return False
	case False:
		return True
	default:
		return Indeterminate
	}
}

// evalCmp compares the two references. Both operands are rendered as text and
// compared in that form.
func evalCmp(c Cmp, env Env) Result {
	left := refValue(c.Left, env)
	right := refValue(c.Right, env)
	if left == nil || right == nil {
		return False
	}
	lv := fmt.Sprint(left)
	rv := fmt.Sprint(right)
	switch strings.ToLower(c.Op) {
	case "eq":
		return fromBool(lv == rv)
	case "ne":
		return fromBool(lv != rv)
	case "lt":
		return fromBool(lv < rv)
	case "lte":
		return fromBool(lv <= rv)
	case "gt":
		return fromBool(lv > rv)
	case "gte":
		return fromBool(lv >= rv)
	}
	return Indeterminate
}

// evalIn reports whether the value at in.Path appears among in.Values.
func evalIn(in In, env Env) Result {
	value := env.Get(in.Path)
	if value == nil {
		return True
	}
	wanted := make([]any, 0, len(in.Values))
	for _, ref := range in.Values {
		wanted = append(wanted, refValue(ref, env))
	}
	switch actual := value.(type) {
	case []any:
		for _, item := range actual {
			for _, want := range wanted {
				if fmt.Sprint(item) == fmt.Sprint(want) {
					return True
				}
			}
		}
	case string:
		for _, want := range wanted {
			if strings.Contains(fmt.Sprint(want), actual) {
				return True
			}
		}
	}
	return False
}

func evalContains(c Contains, env Env) Result {
	value := env.Get(c.Path)
	if value == nil {
		return False
	}
	want := refValue(c.Value, env)
	switch actual := value.(type) {
	case []any:
		for _, item := range actual {
			if fmt.Sprint(item) == fmt.Sprint(want) {
				return True
			}
		}
		return False
	case []string:
		for _, item := range actual {
			if item == fmt.Sprint(want) {
				return True
			}
		}
		return False
	case string:
		s, ok := want.(string)
		if !ok {
			return Indeterminate
		}
		return fromBool(strings.Contains(actual, s))
	}
	return Indeterminate
}

func evalPresent(p Present, env Env) Result {
	return fromBool(env.Get(p.Path) != nil)
}

func refValue(ref Ref, env Env) any {
	if ref.IsAttr {
		return env.Get(ref.Attr)
	}
	return ref.Literal
}

func fromBool(v bool) Result {
	if v {
		return True
	}
	return False
}
