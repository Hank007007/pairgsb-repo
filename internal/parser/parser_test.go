package parser

import (
	"testing"

	"rulelang/internal/ast"
	"rulelang/internal/lexer"
)

func parse(t *testing.T, src string) ast.Expr {
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

func TestParseSingleComparison(t *testing.T) {
	expr := parse(t, `method == "POST"`)
	cmp, ok := expr.(*ast.Compare)
	if !ok {
		t.Fatalf("root is %T, want *ast.Compare", expr)
	}
	if cmp.Op != "==" {
		t.Errorf("operator is %q", cmp.Op)
	}
	field, ok := cmp.Left.(*ast.Field)
	if !ok || field.Name != "method" {
		t.Fatalf("left is %#v", cmp.Left)
	}
	lit, ok := cmp.Right.(*ast.StringLiteral)
	if !ok || lit.Value != "POST" {
		t.Fatalf("right is %#v", cmp.Right)
	}
}

func TestParseSingleAnd(t *testing.T) {
	expr := parse(t, `method == "POST" and body.len > 4096`)
	log, ok := expr.(*ast.Logical)
	if !ok {
		t.Fatalf("root is %T, want *ast.Logical", expr)
	}
	if log.Op != "and" {
		t.Errorf("operator is %q", log.Op)
	}
	if _, ok := log.Left.(*ast.Compare); !ok {
		t.Fatalf("left is %T, want *ast.Compare", log.Left)
	}
	if _, ok := log.Right.(*ast.Compare); !ok {
		t.Fatalf("right is %T, want *ast.Compare", log.Right)
	}
}

func TestParseParentheses(t *testing.T) {
	expr := parse(t, `(method == "POST")`)
	if _, ok := expr.(*ast.Compare); !ok {
		t.Fatalf("root is %T, want *ast.Compare", expr)
	}
}

func TestParseNegation(t *testing.T) {
	expr := parse(t, `not (method == "POST")`)
	neg, ok := expr.(*ast.Not)
	if !ok {
		t.Fatalf("root is %T, want *ast.Not", expr)
	}
	if _, ok := neg.Operand.(*ast.Compare); !ok {
		t.Fatalf("operand is %T, want *ast.Compare", neg.Operand)
	}
}

func TestParseMembership(t *testing.T) {
	expr := parse(t, `src in 10.0.0.0/8`)
	cmp, ok := expr.(*ast.Compare)
	if !ok {
		t.Fatalf("root is %T, want *ast.Compare", expr)
	}
	if cmp.Op != "in" {
		t.Errorf("operator is %q", cmp.Op)
	}
	if _, ok := cmp.Right.(*ast.CIDR); !ok {
		t.Fatalf("right is %T, want *ast.CIDR", cmp.Right)
	}
}

func TestParseRejectsMissingOperand(t *testing.T) {
	toks, err := lexer.Lex(`a ==`)
	if err != nil {
		t.Fatalf("Lex returned error: %v", err)
	}
	if _, err := Parse(toks); err == nil {
		t.Fatal("expected error for missing operand")
	}
}

func TestParseRejectsTrailingTokens(t *testing.T) {
	toks, err := lexer.Lex(`a == 1 b == 2`)
	if err != nil {
		t.Fatalf("Lex returned error: %v", err)
	}
	if _, err := Parse(toks); err == nil {
		t.Fatal("expected error for trailing tokens")
	}
}

func TestParseRejectsLiteralOnLeft(t *testing.T) {
	toks, err := lexer.Lex(`"x" == a`)
	if err != nil {
		t.Fatalf("Lex returned error: %v", err)
	}
	if _, err := Parse(toks); err == nil {
		t.Fatal("expected error for literal on the left")
	}
}

func TestParseRejectsBareField(t *testing.T) {
	toks, err := lexer.Lex(`method`)
	if err != nil {
		t.Fatalf("Lex returned error: %v", err)
	}
	if _, err := Parse(toks); err == nil {
		t.Fatal("expected error for a bare field")
	}
}
