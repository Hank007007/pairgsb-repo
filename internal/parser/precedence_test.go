package parser

import (
	"testing"

	"rulelang/internal/ast"
	"rulelang/internal/lexer"
)

func mustParse(t *testing.T, src string) ast.Expr {
	t.Helper()
	toks, err := lexer.Lex(src)
	if err != nil {
		t.Fatalf("Lex(%q) returned error: %v", src, err)
	}
	expr, err := Parse(toks)
	if err != nil {
		t.Fatalf("Parse(%q) returned error: %v", src, err)
	}
	return expr
}

func TestOrHasLowerPrecedenceThanAnd(t *testing.T) {
	// a == 1 or b == 2 and c == 3 must parse as
	// a == 1 or (b == 2 and c == 3), with "or" at the root.
	expr := mustParse(t, `a == 1 or b == 2 and c == 3`)
	orNode, ok := expr.(*ast.Logical)
	if !ok || orNode.Op != "or" {
		t.Fatalf("root is %#v, want or-node", expr)
	}
	right, ok := orNode.Right.(*ast.Logical)
	if !ok || right.Op != "and" {
		t.Fatalf("right child is %#v, want and-node", orNode.Right)
	}
	if _, ok := orNode.Left.(*ast.Compare); !ok {
		t.Fatalf("left child is %T, want compare", orNode.Left)
	}
}

func TestAndBindsTighterAcrossOrder(t *testing.T) {
	// a and b or c and d => (a and b) or (c and d)
	expr := mustParse(t, `a == 1 and b == 2 or c == 3 and d == 4`)
	orNode, ok := expr.(*ast.Logical)
	if !ok || orNode.Op != "or" {
		t.Fatalf("root is %#v, want or-node", expr)
	}
	if left, ok := orNode.Left.(*ast.Logical); !ok || left.Op != "and" {
		t.Fatalf("left child is %#v, want and-node", orNode.Left)
	}
	if right, ok := orNode.Right.(*ast.Logical); !ok || right.Op != "and" {
		t.Fatalf("right child is %#v, want and-node", orNode.Right)
	}
}

func TestOrIsLeftAssociative(t *testing.T) {
	expr := mustParse(t, `a == 1 or b == 2 or c == 3`)
	orNode, ok := expr.(*ast.Logical)
	if !ok || orNode.Op != "or" {
		t.Fatalf("root is %#v, want or-node", expr)
	}
	if _, ok := orNode.Left.(*ast.Logical); !ok {
		t.Fatalf("left child is %T, want nested or-node", orNode.Left)
	}
	if _, ok := orNode.Right.(*ast.Compare); !ok {
		t.Fatalf("right child is %T, want compare", orNode.Right)
	}
}

func TestNotWrapsWholeComparison(t *testing.T) {
	// not a == b => not (a == b)
	expr := mustParse(t, `not a == b`)
	notNode, ok := expr.(*ast.Not)
	if !ok {
		t.Fatalf("root is %T, want *ast.Not", expr)
	}
	if _, ok := notNode.Operand.(*ast.Compare); !ok {
		t.Fatalf("operand is %T, want compare", notNode.Operand)
	}
}

func TestNotIsRightAssociative(t *testing.T) {
	expr := mustParse(t, `not not a == b`)
	outer, ok := expr.(*ast.Not)
	if !ok {
		t.Fatalf("root is %T, want *ast.Not", expr)
	}
	inner, ok := outer.Operand.(*ast.Not)
	if !ok {
		t.Fatalf("operand is %T, want nested *ast.Not", outer.Operand)
	}
	if _, ok := inner.Operand.(*ast.Compare); !ok {
		t.Fatalf("inner operand is %T, want compare", inner.Operand)
	}
}

func TestNotAndCombination(t *testing.T) {
	// not a == b and c == 3 => (not (a == b)) and (c == 3)
	expr := mustParse(t, `not a == b and c == 3`)
	andNode, ok := expr.(*ast.Logical)
	if !ok || andNode.Op != "and" {
		t.Fatalf("root is %#v, want and-node", expr)
	}
	if _, ok := andNode.Left.(*ast.Not); !ok {
		t.Fatalf("left child is %T, want not-node", andNode.Left)
	}
}

func TestParenthesesOverridePrecedence(t *testing.T) {
	expr := mustParse(t, `(a == 1 or b == 2) and c == 3`)
	andNode, ok := expr.(*ast.Logical)
	if !ok || andNode.Op != "and" {
		t.Fatalf("root is %#v, want and-node", expr)
	}
	if left, ok := andNode.Left.(*ast.Logical); !ok || left.Op != "or" {
		t.Fatalf("left child is %#v, want or-node", andNode.Left)
	}
}

func TestRejectsChainedComparison(t *testing.T) {
	toks, err := lexer.Lex(`a == b == c`)
	if err != nil {
		t.Fatalf("Lex returned error: %v", err)
	}
	if _, err := Parse(toks); err == nil {
		t.Fatal("expected error for chained comparison")
	}
}

func TestRejectsNotOnBareField(t *testing.T) {
	toks, err := lexer.Lex(`not (a)`)
	if err != nil {
		t.Fatalf("Lex returned error: %v", err)
	}
	if _, err := Parse(toks); err == nil {
		t.Fatal("expected error for negating a bare field")
	}
}

func TestParenDepthLimit(t *testing.T) {
	deep := func(n int) string {
		s := ""
		for i := 0; i < n; i++ {
			s += "("
		}
		s += "a == 1"
		for i := 0; i < n; i++ {
			s += ")"
		}
		return s
	}

	// 64 nested levels must still compile.
	mustParse(t, deep(64))

	// The 65th unclosed parenthesis must be rejected at parse time.
	toks, err := lexer.Lex(deep(65))
	if err != nil {
		t.Fatalf("Lex returned error: %v", err)
	}
	if _, err := Parse(toks); err == nil {
		t.Fatal("expected error for parenthesis depth 65")
	}
}
