// Package eval applies a compiled rule to a request context.
package eval

import (
	"fmt"
	"net/netip"
	"strconv"

	"rulelang/internal/ast"
	"rulelang/internal/rlerr"
)

// Kind describes the runtime type of a field value.
type Kind int

const (
	// KindString is a UTF-8 string value.
	KindString Kind = iota
	// KindInt is a signed 64-bit integer value.
	KindInt
	// KindIP is an IPv4 address held as dotted-decimal text.
	KindIP
)

// Value is a single request attribute value.
type Value struct {
	Kind Kind
	Str  string
	Int  int64
}

// StringValue builds a string value.
func StringValue(s string) Value { return Value{Kind: KindString, Str: s} }

// IntValue builds an integer value.
func IntValue(n int64) Value { return Value{Kind: KindInt, Int: n} }

// IPValue builds an IP address value.
func IPValue(s string) Value { return Value{Kind: KindIP, Str: s} }

// Text renders the value the way it appears in a rule.
func (v Value) Text() string {
	if v.Kind == KindInt {
		return strconv.FormatInt(v.Int, 10)
	}
	return v.Str
}

// Context maps field names to values.
type Context map[string]Value

// Eval evaluates a compiled rule against ctx.
func Eval(expr ast.Expr, ctx Context) (bool, error) {
	if expr == nil {
		return false, rlerr.New(rlerr.StageEval, 0, "cannot evaluate an empty rule")
	}
	switch node := expr.(type) {
	case *ast.Logical:
		if node == nil || node.Left == nil || node.Right == nil {
			return false, rlerr.New(rlerr.StageEval, node.Pos(), "logical expression is incomplete")
		}
		left, err := Eval(node.Left, ctx)
		if err != nil {
			return false, err
		}
		switch node.Op {
		case "and":
			if !left {
				return false, nil
			}
			return Eval(node.Right, ctx)
		case "or":
			if left {
				return true, nil
			}
			return Eval(node.Right, ctx)
		}
		return false, rlerr.New(rlerr.StageEval, node.Pos(), "unknown logical operator "+node.Op)

	case *ast.Not:
		if node == nil || node.Operand == nil {
			return false, rlerr.New(rlerr.StageEval, node.Pos(), "negation is incomplete")
		}
		value, err := Eval(node.Operand, ctx)
		if err != nil {
			return false, err
		}
		return !value, nil

	case *ast.Compare:
		if node == nil || node.Left == nil || node.Right == nil {
			return false, rlerr.New(rlerr.StageEval, node.Pos(), "comparison is incomplete")
		}
		return evalCompare(node, ctx)
	}

	return false, rlerr.New(rlerr.StageEval, expr.Pos(), "cannot evaluate this expression")
}

func evalCompare(node *ast.Compare, ctx Context) (bool, error) {
	if node.Op == "in" {
		return evalIn(node, ctx)
	}

	left, err := resolve(node.Left, ctx)
	if err != nil {
		return false, err
	}
	right, err := resolve(node.Right, ctx)
	if err != nil {
		return false, err
	}

	switch node.Op {
	case "==", "!=":
		if left.Kind != right.Kind {
			return false, rlerr.New(rlerr.StageEval, node.Pos(), "cannot compare values of different types")
		}
		var same bool
		if left.Kind == KindInt {
			same = left.Int == right.Int
		} else {
			same = left.Str == right.Str
		}
		if node.Op == "!=" {
			return !same, nil
		}
		return same, nil

	case "<", "<=", ">", ">=":
		if left.Kind != KindInt || right.Kind != KindInt {
			return false, rlerr.New(rlerr.StageEval, node.Pos(), "ordering comparison requires integer values")
		}
		switch node.Op {
		case "<":
			return left.Int < right.Int, nil
		case "<=":
			return left.Int <= right.Int, nil
		case ">":
			return left.Int > right.Int, nil
		default:
			return left.Int >= right.Int, nil
		}
	}

	return false, rlerr.New(rlerr.StageEval, node.Pos(), "unknown comparison operator "+node.Op)
}

func evalIn(node *ast.Compare, ctx Context) (bool, error) {
	left, err := resolve(node.Left, ctx)
	if err != nil {
		return false, err
	}

	if cidr, ok := node.Right.(*ast.CIDR); ok {
		return matchCIDR(left.Text(), cidr.Text, node.Pos())
	}

	right, err := resolve(node.Right, ctx)
	if err != nil {
		return false, err
	}
	return left.Text() == right.Text(), nil
}

// matchCIDR reports whether ip falls inside the network written in cidr. The
// decision is made on the binary network prefix, not on textual prefixes of
// the address. A malformed address or network is an evaluation error.
func matchCIDR(ip, cidr string, pos int) (bool, error) {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false, rlerr.New(rlerr.StageEval, pos, "invalid ip address "+ip)
	}
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return false, rlerr.New(rlerr.StageEval, pos, "invalid cidr "+cidr)
	}
	return prefix.Contains(addr), nil
}

func resolve(n ast.Node, ctx Context) (Value, error) {
	if n == nil {
		return Value{}, rlerr.New(rlerr.StageEval, 0, "missing operand")
	}
	switch v := n.(type) {
	case *ast.Field:
		value, ok := ctx[v.Name]
		if !ok {
			return Value{}, rlerr.New(rlerr.StageEval, v.Pos(), "field "+v.Name+" is not present in the request context")
		}
		return value, nil
	case *ast.StringLiteral:
		return StringValue(v.Value), nil
	case *ast.IntLiteral:
		return IntValue(v.Value), nil
	}
	return Value{}, rlerr.New(rlerr.StageEval, n.Pos(), fmt.Sprintf("cannot use %s as an operand", n.Kind()))
}
