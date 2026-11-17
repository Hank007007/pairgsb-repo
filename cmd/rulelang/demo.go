package main

import (
	"fmt"
	"io"
	"strconv"

	"rulelang/internal/ast"
	"rulelang/internal/compiler"
	"rulelang/internal/eval"
)

// demo compiles a few illustrative rules and evaluates them against request
// contexts whose field values are chosen so the outcome is visible.
func demo(stdout io.Writer) error {
	fmt.Fprintln(stdout, "== precedence: a == 1 or b == 2 and c == 3 ==")
	if err := runDemoRule("a == 1 or b == 2 and c == 3", []eval.Context{
		{"a": eval.IntValue(1), "b": eval.IntValue(0), "c": eval.IntValue(0)},
		{"a": eval.IntValue(0), "b": eval.IntValue(2), "c": eval.IntValue(3)},
		{"a": eval.IntValue(0), "b": eval.IntValue(2), "c": eval.IntValue(0)},
		{"a": eval.IntValue(1), "b": eval.IntValue(2), "c": eval.IntValue(0)},
	}, stdout); err != nil {
		return err
	}

	fmt.Fprintln(stdout, "== negation: not a == b ==")
	if err := runDemoRule("not a == b", []eval.Context{
		{"a": eval.IntValue(5), "b": eval.IntValue(5)},
		{"a": eval.IntValue(5), "b": eval.IntValue(3)},
	}, stdout); err != nil {
		return err
	}

	fmt.Fprintln(stdout, "== membership: src in 10.0.0.0/8 ==")
	if err := runDemoRule("src in 10.0.0.0/8", []eval.Context{
		{"src": eval.IPValue("10.7.7.7")},
		{"src": eval.IPValue("100.1.2.3")},
	}, stdout); err != nil {
		return err
	}

	fmt.Fprintln(stdout, "== string escapes: path == \"a\\\\b\\n\\t\\\"\" ==")
	if err := runDemoRule(`path == "a\\b\n\t\""`, []eval.Context{
		{"path": eval.StringValue("a\\b\n\t\"")},
	}, stdout); err != nil {
		return err
	}

	return nil
}

func runDemoRule(src string, contexts []eval.Context, stdout io.Writer) error {
	expr, err := compiler.Compile(src)
	if err != nil {
		fmt.Fprintf(stdout, "compile error: %v\n\n", err)
		return nil
	}
	fmt.Fprintln(stdout, ast.Format(expr))
	for _, ctx := range contexts {
		desc := describe(ctx)
		result, err := eval.Eval(expr, ctx)
		if err != nil {
			fmt.Fprintf(stdout, "  %-46s -> error: %v\n", desc, err)
			continue
		}
		fmt.Fprintf(stdout, "  %-46s -> %v\n", desc, result)
	}
	fmt.Fprintln(stdout)
	return nil
}

func describe(ctx eval.Context) string {
	order := []string{"a", "b", "c", "method", "body.len", "src", "path"}
	parts := ""
	for _, name := range order {
		v, ok := ctx[name]
		if !ok {
			continue
		}
		if parts != "" {
			parts += ", "
		}
		if v.Kind == eval.KindInt {
			parts += name + "=" + v.Text()
		} else {
			parts += name + "=" + strconv.Quote(v.Text())
		}
	}
	if parts == "" {
		return "(empty)"
	}
	return parts
}
