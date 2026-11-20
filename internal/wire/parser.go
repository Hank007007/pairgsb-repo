package wire

import "ptrloop/internal/limits"

// Parser decodes a stream of upstream responses that belong to the same query
// path. A resolver keeps one Parser per in-flight upstream conversation, so the
// same budget is reused from one response to the next.
type Parser struct {
	budget *limits.Budget
}

// NewParser returns a parser with a fresh budget.
func NewParser() *Parser {
	return &Parser{budget: limits.New()}
}

// Parse decodes msg using the parser's shared budget.
func (p *Parser) Parse(msg []byte) (*Message, error) {
	return Parse(msg, p.budget)
}

// Budget exposes the parser's counters so callers can log them.
func (p *Parser) Budget() *limits.Budget {
	return p.budget
}
