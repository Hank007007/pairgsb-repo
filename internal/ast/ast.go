// Package ast defines the node types produced by the rule compiler.
//
// The tree is evaluated by internal/eval. Every node carries the byte offset
// of the source construct that produced it so diagnostics can point at the
// offending part of a rule.
package ast

import "fmt"

// Node is the common interface implemented by every expression node.
type Node interface {
	// Pos reports the byte offset in the rule source where the node begins.
	Pos() int
	// Kind is a short stable name used in diagnostics and tree dumps.
	Kind() string
}

// Expr is any boolean-valued expression.
type Expr interface {
	Node
	exprNode()
}

// Field references a request attribute by name, e.g. method or body.len.
type Field struct {
	Name string
	Off  int
}

// StringLiteral is a decoded string literal.
type StringLiteral struct {
	Value string
	Off   int
}

// IntLiteral is a decimal integer literal that fits in int64.
type IntLiteral struct {
	Value int64
	Off   int
}

// CIDR is an IPv4 network written in prefix notation, e.g. 10.0.0.0/8.
type CIDR struct {
	Text string
	Off  int
}

// Compare is a binary comparison or membership test. Operands are normally
// fields or literals; the general Node type keeps the tree shape visible even
// when a sub-expression appears in operand position.
type Compare struct {
	Op    string // == != < <= > >= in
	Left  Node
	Right Node
	Off   int
}

// Logical is an and/or node.
type Logical struct {
	Op    string // and, or
	Left  Expr
	Right Expr
	Off   int
}

// Not is a logical negation.
type Not struct {
	Operand Expr
	Off     int
}

// Pos implementations.

func (n *Field) Pos() int         { return n.Off }
func (n *StringLiteral) Pos() int { return n.Off }
func (n *IntLiteral) Pos() int    { return n.Off }
func (n *CIDR) Pos() int          { return n.Off }
func (n *Compare) Pos() int       { return n.Off }
func (n *Logical) Pos() int       { return n.Off }
func (n *Not) Pos() int           { return n.Off }

// Kind implementations.

func (n *Field) Kind() string         { return "field" }
func (n *StringLiteral) Kind() string { return "string" }
func (n *IntLiteral) Kind() string    { return "int" }
func (n *CIDR) Kind() string          { return "cidr" }
func (n *Compare) Kind() string       { return "compare" }
func (n *Logical) Kind() string       { return n.Op }
func (n *Not) Kind() string           { return "not" }

func (n *Field) exprNode()         {}
func (n *StringLiteral) exprNode() {}
func (n *IntLiteral) exprNode()    {}
func (n *CIDR) exprNode()          {}
func (n *Compare) exprNode()       {}
func (n *Logical) exprNode()       {}
func (n *Not) exprNode()           {}

// Count returns the number of nodes in the tree rooted at n. A nil root has
// zero nodes.
func Count(n Node) int {
	if n == nil {
		return 0
	}
	switch v := n.(type) {
	case *Field, *StringLiteral, *IntLiteral, *CIDR:
		return 1
	case *Compare:
		return 1 + Count(v.Left) + Count(v.Right)
	case *Logical:
		return 1 + Count(v.Left) + Count(v.Right)
	case *Not:
		return 1 + Count(v.Operand)
	}
	return 1
}

// Format renders a tree as an indented s-expression, used by the CLI demo.
func Format(n Node) string {
	return format(n, 0)
}

func format(n Node, depth int) string {
	if n == nil {
		return "<nil>"
	}
	pad := ""
	for i := 0; i < depth; i++ {
		pad += "  "
	}
	switch v := n.(type) {
	case *Field:
		return fmt.Sprintf("%s(field %s)", pad, v.Name)
	case *StringLiteral:
		return fmt.Sprintf("%s(string %q)", pad, v.Value)
	case *IntLiteral:
		return fmt.Sprintf("%s(int %d)", pad, v.Value)
	case *CIDR:
		return fmt.Sprintf("%s(cidr %s)", pad, v.Text)
	case *Compare:
		return fmt.Sprintf("%s(compare %s\n%s\n%s)", pad, v.Op, format(v.Left, depth+1), format(v.Right, depth+1))
	case *Logical:
		return fmt.Sprintf("%s(%s\n%s\n%s)", pad, v.Op, format(v.Left, depth+1), format(v.Right, depth+1))
	case *Not:
		return fmt.Sprintf("%s(not\n%s)", pad, format(v.Operand, depth+1))
	}
	return pad + "<unknown>"
}
