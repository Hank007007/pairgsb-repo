package eval

import (
	"testing"

	"rulelang/internal/ast"
)

func intCtx(pairs ...any) Context {
	ctx := Context{}
	for i := 0; i < len(pairs); i += 2 {
		ctx[pairs[i].(string)] = IntValue(pairs[i+1].(int64))
	}
	return ctx
}

func TestEvalPrecedenceOrVsAnd(t *testing.T) {
	src := `a == 1 or b == 2 and c == 3`
	cases := []struct {
		ctx  Context
		want bool
	}{
		{intCtx("a", int64(1), "b", int64(0), "c", int64(0)), true},
		{intCtx("a", int64(1), "b", int64(2), "c", int64(0)), true},
		{intCtx("a", int64(0), "b", int64(2), "c", int64(3)), true},
		{intCtx("a", int64(0), "b", int64(2), "c", int64(0)), false},
		{intCtx("a", int64(0), "b", int64(0), "c", int64(0)), false},
	}
	for _, tc := range cases {
		if got := mustEval(t, src, tc.ctx); got != tc.want {
			t.Errorf("Eval(%v) = %v, want %v", tc.ctx, got, tc.want)
		}
	}
}

func TestEvalNotWrapsComparison(t *testing.T) {
	src := `not a == b`
	if !mustEval(t, src, intCtx("a", int64(5), "b", int64(3))) {
		t.Error("expected true when a and b differ")
	}
	if mustEval(t, src, intCtx("a", int64(5), "b", int64(5))) {
		t.Error("expected false when a and b are equal")
	}
}

func TestEvalCIDRUsesBinaryPrefix(t *testing.T) {
	src := `src in 10.0.0.0/8`
	if mustEval(t, src, Context{"src": IPValue("100.1.2.3")}) {
		t.Error("100.1.2.3 must not match 10.0.0.0/8")
	}
	if !mustEval(t, src, Context{"src": IPValue("10.7.7.7")}) {
		t.Error("10.7.7.7 must match 10.0.0.0/8")
	}
	// A textual prefix that happens to share leading digits must not count.
	if mustEval(t, `src in 192.168.1.0/24`, Context{"src": IPValue("192.168.100.1")}) {
		t.Error("192.168.100.1 must not match 192.168.1.0/24")
	}
	if !mustEval(t, `src in 192.168.1.0/24`, Context{"src": IPValue("192.168.1.99")}) {
		t.Error("192.168.1.99 must match 192.168.1.0/24")
	}
}

func TestEvalMalformedIPIsErrorNotPanic(t *testing.T) {
	expr := compile(t, `src in 10.0.0.0/8`)
	if _, err := Eval(expr, Context{"src": IPValue("not-an-ip")}); err == nil {
		t.Fatal("expected an evaluation error for a malformed ip")
	}
}

func TestEvalNilTreeReturnsError(t *testing.T) {
	b, err := Eval(nil, Context{})
	if err == nil {
		t.Fatalf("expected error, got result %v", b)
	}
}

func TestEvalNilChildrenReturnError(t *testing.T) {
	cases := []ast.Expr{
		&ast.Not{},
		&ast.Logical{},
		&ast.Compare{},
	}
	for _, expr := range cases {
		b, err := Eval(expr, Context{})
		if err == nil {
			t.Errorf("%T: expected error, got result %v", expr, b)
		}
	}
}
