package eval

import (
	"testing"

	"rulelang/internal/ast"
	"rulelang/internal/compiler"
)

func compile(t *testing.T, src string) ast.Expr {
	t.Helper()
	expr, err := compiler.Compile(src)
	if err != nil {
		t.Fatalf("Compile(%q) returned error: %v", src, err)
	}
	return expr
}

func mustEval(t *testing.T, src string, ctx Context) bool {
	t.Helper()
	result, err := Eval(compile(t, src), ctx)
	if err != nil {
		t.Fatalf("Eval(%q) returned error: %v", src, err)
	}
	return result
}

func TestEvalStringEquality(t *testing.T) {
	ctx := Context{"method": StringValue("POST")}
	if !mustEval(t, `method == "POST"`, ctx) {
		t.Error("expected true for matching method")
	}
	if mustEval(t, `method == "GET"`, ctx) {
		t.Error("expected false for non-matching method")
	}
}

func TestEvalIntegerComparison(t *testing.T) {
	ctx := Context{"body.len": IntValue(8192)}
	if !mustEval(t, `body.len > 4096`, ctx) {
		t.Error("expected true for body.len > 4096")
	}
	if mustEval(t, `body.len < 4096`, ctx) {
		t.Error("expected false for body.len < 4096")
	}
}

func TestEvalSingleAnd(t *testing.T) {
	ctx := Context{
		"method":   StringValue("POST"),
		"body.len": IntValue(8192),
	}
	if !mustEval(t, `method == "POST" and body.len > 4096`, ctx) {
		t.Error("expected true when both sides hold")
	}
	ctx["body.len"] = IntValue(512)
	if mustEval(t, `method == "POST" and body.len > 4096`, ctx) {
		t.Error("expected false when one side fails")
	}
}

func TestEvalShortCircuit(t *testing.T) {
	ctx := Context{"method": StringValue("GET")}
	if mustEval(t, `method == "POST" and missing == 1`, ctx) {
		t.Error("expected false")
	}
	if !mustEval(t, `method == "GET" or missing == 1`, ctx) {
		t.Error("expected true")
	}
}

func TestEvalMissingField(t *testing.T) {
	if _, err := Eval(compile(t, `absent == "x"`), Context{}); err == nil {
		t.Fatal("expected an error for a missing field")
	}
}

func TestEvalNot(t *testing.T) {
	ctx := Context{"method": StringValue("GET")}
	if !mustEval(t, `not (method == "POST")`, ctx) {
		t.Error("expected true")
	}
}

func TestEvalTypeMismatch(t *testing.T) {
	ctx := Context{"body.len": IntValue(10)}
	if _, err := Eval(compile(t, `body.len == "10"`), ctx); err == nil {
		t.Fatal("expected an error comparing int to string")
	}
}
