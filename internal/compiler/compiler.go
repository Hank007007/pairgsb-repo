// Package compiler ties the lexical and syntactic stages together.
package compiler

import (
	"rulelang/internal/ast"
	"rulelang/internal/lexer"
	"rulelang/internal/parser"
)

// Compile turns rule source text into an expression tree. A rule that fails to
// compile must not be handed to the evaluator.
func Compile(src string) (ast.Expr, error) {
	toks, err := lexer.Lex(src)
	if err != nil {
		return nil, err
	}
	return parser.Parse(toks)
}
