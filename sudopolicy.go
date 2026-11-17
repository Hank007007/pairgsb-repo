// Package sudopolicy decides whether a command submitted to a bastion host
// executor may run.
//
// A caller supplies a parsed policy table and a [Request] describing the
// operator, the target host, the argument vector, the environment variables
// the operator wants to preserve and whether a password has been supplied.
// [Evaluate] returns a [Result] carrying the decision.
//
// The configuration syntax is documented in docs/policy-grammar.md and the
// behaviour of the evaluator in docs/design.md.
package sudopolicy

import (
	"io"
	"os"

	"sudopolicy/internal/eval"
	"sudopolicy/internal/parse"
	"sudopolicy/internal/policy"
)

// Table is an ordered set of authorization rules.
type Table = policy.Table

// Entry is a single authorization rule.
type Entry = policy.Entry

// Request describes a command execution to authorize.
type Request = policy.Request

// Result is the outcome of an evaluation.
type Result = policy.Result

// Load reads and parses the policy file at path.
func Load(path string) (*Table, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return Parse(file)
}

// Parse reads a policy from r.
func Parse(r io.Reader) (*Table, error) {
	return parse.Parse(r)
}

// Evaluate applies table to request.
func Evaluate(table *Table, request Request) (Result, error) {
	return eval.Evaluate(table, request)
}
