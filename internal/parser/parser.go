// Package parser builds an expression tree from the token stream produced by
// package lexer.
package parser

import (
	"rulelang/internal/ast"
	"rulelang/internal/lexer"
	"rulelang/internal/limits"
	"rulelang/internal/rlerr"
)

// Parse consumes the token stream and returns the tree for a complete rule.
// The stream must end with an EOF token.
func Parse(toks []lexer.Token) (ast.Expr, error) {
	p := &parser{toks: toks}
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if tok := p.peek(); tok.Kind != lexer.EOF {
		return nil, unexpected(tok, "end of rule")
	}
	if !isBoolExpr(expr) {
		return nil, rlerr.New(rlerr.StageParse, expr.Pos(), "rule must be a boolean expression")
	}
	return expr, nil
}

type parser struct {
	toks  []lexer.Token
	pos   int
	nodes int
}

func (p *parser) peek() lexer.Token {
	if p.pos >= len(p.toks) {
		return lexer.Token{Kind: lexer.EOF}
	}
	return p.toks[p.pos]
}

func (p *parser) next() lexer.Token {
	tok := p.peek()
	if p.pos < len(p.toks) {
		p.pos++
	}
	return tok
}

// bump accounts for one more AST node and enforces the node budget.
func (p *parser) bump(pos int) error {
	p.nodes++
	if p.nodes > limits.MaxASTNodes {
		return rlerr.New(rlerr.StageLimit, pos, "expression has too many nodes")
	}
	return nil
}

func (p *parser) parseExpr() (ast.Expr, error) {
	return p.parseBinary()
}

// parseBinary parses a chain of boolean operators.
func (p *parser) parseBinary() (ast.Expr, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	if !isBoolExpr(left) {
		return nil, rlerr.New(rlerr.StageParse, left.Pos(), "expected a comparison, got "+left.Kind())
	}
	for {
		kind := p.peek().Kind
		if kind != lexer.And && kind != lexer.Or {
			return left, nil
		}
		op := p.next()
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		if !isBoolExpr(right) {
			return nil, rlerr.New(rlerr.StageParse, right.Pos(), "expected a comparison, got "+right.Kind())
		}
		if err := p.bump(op.Pos); err != nil {
			return nil, err
		}
		left = &ast.Logical{Op: op.Text, Left: left, Right: right, Off: op.Pos}
	}
}

// parseComparison parses an optional comparison or membership operator between
// two unary expressions.
func (p *parser) parseComparison() (ast.Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	kind := p.peek().Kind
	if !isCompareOp(kind) {
		return left, nil
	}
	op := p.next()
	right, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	if isLiteral(left) {
		return nil, rlerr.New(rlerr.StageParse, left.Pos(), "left operand of "+op.Text+" must be a field")
	}
	if right.Kind() == "cidr" && op.Kind != lexer.In {
		return nil, rlerr.New(rlerr.StageParse, right.Pos(), "cidr literal is only valid with 'in'")
	}
	if err := p.bump(op.Pos); err != nil {
		return nil, err
	}
	return &ast.Compare{Op: op.Text, Left: left, Right: right, Off: op.Pos}, nil
}

// parseUnary parses a logical negation or falls through to a primary.
func (p *parser) parseUnary() (ast.Expr, error) {
	if p.peek().Kind == lexer.Not {
		tok := p.next()
		operand, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		if err := p.bump(tok.Pos); err != nil {
			return nil, err
		}
		return &ast.Not{Operand: operand, Off: tok.Pos}, nil
	}
	return p.parsePrimary()
}

// parsePrimary parses a parenthesized expression or a single operand.
func (p *parser) parsePrimary() (ast.Expr, error) {
	tok := p.peek()
	switch tok.Kind {
	case lexer.LParen:
		p.next()
		inner, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.peek().Kind != lexer.RParen {
			return nil, unexpected(p.peek(), "')'")
		}
		p.next()
		return inner, nil

	case lexer.Ident:
		p.next()
		if err := p.bump(tok.Pos); err != nil {
			return nil, err
		}
		return &ast.Field{Name: tok.Text, Off: tok.Pos}, nil

	case lexer.String:
		p.next()
		if err := p.bump(tok.Pos); err != nil {
			return nil, err
		}
		return &ast.StringLiteral{Value: tok.Text, Off: tok.Pos}, nil

	case lexer.Int:
		p.next()
		if err := p.bump(tok.Pos); err != nil {
			return nil, err
		}
		return &ast.IntLiteral{Value: tok.IntVal, Off: tok.Pos}, nil

	case lexer.CIDR:
		p.next()
		if err := p.bump(tok.Pos); err != nil {
			return nil, err
		}
		return &ast.CIDR{Text: tok.Text, Off: tok.Pos}, nil
	}

	return nil, unexpected(tok, "expression")
}

func isCompareOp(k lexer.Kind) bool {
	switch k {
	case lexer.Eq, lexer.Ne, lexer.Lt, lexer.Le, lexer.Gt, lexer.Ge, lexer.In:
		return true
	}
	return false
}

func isLiteral(n ast.Node) bool {
	switch n.(type) {
	case *ast.StringLiteral, *ast.IntLiteral, *ast.CIDR:
		return true
	}
	return false
}

func isBoolExpr(n ast.Node) bool {
	switch n.(type) {
	case *ast.Compare, *ast.Logical, *ast.Not:
		return true
	}
	return false
}

func unexpected(tok lexer.Token, want string) error {
	got := tok.Kind.String()
	if tok.Kind == lexer.Ident {
		got = "identifier " + tok.Text
	}
	return rlerr.New(rlerr.StageParse, tok.Pos, "expected "+want+", got "+got)
}
