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
	depth int
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

// expr := orExpr
func (p *parser) parseExpr() (ast.Expr, error) {
	return p.parseOr()
}

// orExpr := andExpr ( "or" andExpr )*
func (p *parser) parseOr() (ast.Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().Kind == lexer.Or {
		op := p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		if err := p.bump(op.Pos); err != nil {
			return nil, err
		}
		left = &ast.Logical{Op: op.Text, Left: left, Right: right, Off: op.Pos}
	}
	return left, nil
}

// andExpr := notExpr ( "and" notExpr )*
func (p *parser) parseAnd() (ast.Expr, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.peek().Kind == lexer.And {
		op := p.next()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		if err := p.bump(op.Pos); err != nil {
			return nil, err
		}
		left = &ast.Logical{Op: op.Text, Left: left, Right: right, Off: op.Pos}
	}
	return left, nil
}

// notExpr := "not" notExpr | primary
//
// A negation wraps the whole comparison that follows it, so "not a == b"
// parses as "not (a == b)".
func (p *parser) parseNot() (ast.Expr, error) {
	if p.peek().Kind != lexer.Not {
		return p.parsePrimary()
	}
	tok := p.next()
	operand, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	if !isBoolExpr(operand) {
		return nil, rlerr.New(rlerr.StageParse, operand.Pos(), "not must wrap a boolean expression, got "+operand.Kind())
	}
	if err := p.bump(tok.Pos); err != nil {
		return nil, err
	}
	return &ast.Not{Operand: operand, Off: tok.Pos}, nil
}

// primary := "(" expr ")" | comparison
// comparison := operand cmpOp operand
// operand := ident | string | integer | cidr
func (p *parser) parsePrimary() (ast.Expr, error) {
	if p.peek().Kind == lexer.LParen {
		tok := p.next()
		p.depth++
		if p.depth > limits.MaxParenDepth {
			return nil, rlerr.New(rlerr.StageLimit, tok.Pos, "parenthesis nesting is too deep")
		}
		inner, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.peek().Kind != lexer.RParen {
			return nil, unexpected(p.peek(), "')'")
		}
		p.next()
		p.depth--
		return inner, nil
	}
	return p.parseComparison()
}

// parseComparison parses "operand cmpOp operand". The left operand must be a
// field reference; a literal there is rejected.
func (p *parser) parseComparison() (ast.Expr, error) {
	leftTok := p.peek()
	left, err := p.parseOperand()
	if err != nil {
		return nil, err
	}
	if !isCompareOp(p.peek().Kind) {
		return left, nil
	}
	op := p.next()
	right, err := p.parseOperand()
	if err != nil {
		return nil, err
	}
	if leftTok.Kind != lexer.Ident {
		return nil, rlerr.New(rlerr.StageParse, leftTok.Pos, "left operand of "+op.Text+" must be a field")
	}
	if right.Kind() == "cidr" && op.Kind != lexer.In {
		return nil, rlerr.New(rlerr.StageParse, right.Pos(), "cidr literal is only valid with 'in'")
	}
	if err := p.bump(op.Pos); err != nil {
		return nil, err
	}
	return &ast.Compare{Op: op.Text, Left: left, Right: right, Off: op.Pos}, nil
}

// parseOperand parses a single operand token.
func (p *parser) parseOperand() (ast.Expr, error) {
	tok := p.peek()
	switch tok.Kind {
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
