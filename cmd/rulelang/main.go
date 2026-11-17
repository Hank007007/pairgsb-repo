// Command rulelang compiles a rule expression and optionally evaluates it
// against a request context supplied as JSON.
//
// Usage:
//
//	rulelang compile -rule 'method == "POST"'
//	rulelang eval -rule 'method == "POST"' -ctx '{"method":"POST"}'
//	rulelang eval -rule-file rule.txt -ctx-file ctx.json -ip src
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"rulelang/internal/ast"
	"rulelang/internal/compiler"
	"rulelang/internal/eval"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "rulelang:", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: rulelang <compile|eval|demo> [flags]")
	}
	cmd := args[0]
	if cmd == "demo" {
		return demo(stdout)
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stdout)
	rule := fs.String("rule", "", "rule source text")
	ruleFile := fs.String("rule-file", "", "read rule source from a file")
	ctxJSON := fs.String("ctx", "", "request context as a JSON object")
	ctxFile := fs.String("ctx-file", "", "read request context JSON from a file")
	ipFields := fs.String("ip", "src", "comma-separated field names holding IP addresses")

	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	src, err := readRule(*rule, *ruleFile, stdin)
	if err != nil {
		return err
	}

	switch cmd {
	case "compile":
		expr, err := compiler.Compile(src)
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, ast.Format(expr))
		fmt.Fprintf(stdout, "nodes: %d\n", ast.Count(expr))
		return nil

	case "eval":
		data, err := readContext(*ctxJSON, *ctxFile)
		if err != nil {
			return err
		}
		ctx, err := buildContext(data, *ipFields)
		if err != nil {
			return err
		}
		expr, err := compiler.Compile(src)
		if err != nil {
			return err
		}
		result, err := eval.Eval(expr, ctx)
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, result)
		return nil
	}

	return fmt.Errorf("unknown command %q", cmd)
}

func readRule(inline, file string, stdin io.Reader) (string, error) {
	if inline != "" && file != "" {
		return "", fmt.Errorf("-rule and -rule-file are mutually exclusive")
	}
	if inline != "" {
		return inline, nil
	}
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", fmt.Errorf("no rule source: pass -rule, -rule-file, or stdin")
	}
	return strings.TrimSpace(string(data)), nil
}

func readContext(inline, file string) ([]byte, error) {
	if inline != "" && file != "" {
		return nil, fmt.Errorf("-ctx and -ctx-file are mutually exclusive")
	}
	if inline != "" {
		return []byte(inline), nil
	}
	if file != "" {
		return os.ReadFile(file)
	}
	return []byte("{}"), nil
}

func buildContext(data []byte, ipFields string) (eval.Context, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid context json: %w", err)
	}
	isIP := map[string]bool{}
	for _, name := range strings.Split(ipFields, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			isIP[name] = true
		}
	}
	ctx := eval.Context{}
	for name, value := range raw {
		switch v := value.(type) {
		case string:
			if isIP[name] {
				ctx[name] = eval.IPValue(v)
			} else {
				ctx[name] = eval.StringValue(v)
			}
		case float64:
			ctx[name] = eval.IntValue(int64(v))
		case bool:
			return nil, fmt.Errorf("context field %q is boolean; rule fields are string, integer, or ip", name)
		case nil:
			return nil, fmt.Errorf("context field %q is null", name)
		default:
			return nil, fmt.Errorf("context field %q has unsupported type", name)
		}
	}
	return ctx, nil
}
