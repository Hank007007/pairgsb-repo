package schema

import (
	"fmt"
	"strings"
)

// Parse compiles schema text into an immutable schema.
func Parse(input string) (*Schema, error) {
	tokens, err := tokenize(input)
	if err != nil {
		return nil, err
	}
	p := &parser{tokens: tokens, input: []rune(input)}
	s, err := p.parseSchema()
	if err != nil {
		return nil, err
	}
	return s, nil
}

type parser struct {
	tokens []token
	input  []rune
	pos    int
}

func (p *parser) peek() token {
	return p.tokens[p.pos]
}

func (p *parser) next() token {
	tok := p.tokens[p.pos]
	if tok.kind != tokenEOF {
		p.pos++
	}
	return tok
}

func (p *parser) expect(kind tokenKind, what string) (token, error) {
	tok := p.peek()
	if tok.kind != kind {
		return tok, fmt.Errorf("schema: expected %s at offset %d, found %q", what, tok.pos, tok.text)
	}
	return p.next(), nil
}

func (p *parser) expectIdent(what string) (string, error) {
	tok := p.peek()
	if tok.kind != tokenIdent {
		return "", fmt.Errorf("schema: expected %s at offset %d, found %q", what, tok.pos, tok.text)
	}
	p.next()
	return tok.text, nil
}

func (p *parser) expectKeyword(word string) error {
	tok := p.peek()
	if tok.kind != tokenKeyword || tok.text != word {
		return fmt.Errorf("schema: expected %q at offset %d, found %q", word, tok.pos, tok.text)
	}
	p.next()
	return nil
}

func (p *parser) parseSchema() (*Schema, error) {
	s := &Schema{types: make(map[string]*Type)}

	for p.peek().kind != tokenEOF {
		if err := p.expectKeyword("type"); err != nil {
			return nil, err
		}
		name, err := p.expectIdent("type name")
		if err != nil {
			return nil, err
		}
		if _, exists := s.types[name]; exists {
			return nil, fmt.Errorf("schema: type %q is declared more than once", name)
		}
		if _, err := p.expect(tokenLBrace, "'{'"); err != nil {
			return nil, err
		}

		t := &Type{Name: name, Relations: make(map[string]*Relation)}
		for p.peek().kind != tokenRBrace {
			if p.peek().kind == tokenEOF {
				return nil, fmt.Errorf("schema: type %q is not closed", name)
			}
			if err := p.expectKeyword("relation"); err != nil {
				return nil, err
			}
			relName, err := p.expectIdent("relation name")
			if err != nil {
				return nil, err
			}
			if _, exists := t.Relations[relName]; exists {
				return nil, fmt.Errorf("schema: relation %q is declared more than once in type %q", relName, name)
			}
			if _, err := p.expect(tokenEquals, "'='"); err != nil {
				return nil, err
			}

			start := p.peek().pos
			node, err2 := p.parseExpr()
			if err2 != nil {
				return nil, err2
			}
			end := p.peek().pos
			raw := strings.TrimSpace(string(p.input[start:end]))

			t.Relations[relName] = &Relation{
				Name:     relName,
				Rewrite:  node,
				Raw:      raw,
				Position: start,
			}
			t.Order = append(t.Order, relName)
		}
		if _, err := p.expect(tokenRBrace, "'}'"); err != nil {
			return nil, err
		}

		s.types[name] = t
		s.order = append(s.order, name)
	}

	if len(s.order) == 0 {
		return nil, fmt.Errorf("schema: no type declarations")
	}
	return s, nil
}

// parseExpr parses a full expression including the lowest-precedence union
// operator.
func (p *parser) parseExpr() (Node, error) {
	first, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	children := []Node{first}
	for p.peek().kind == tokenPipe {
		p.next()
		node, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		children = append(children, node)
	}
	if len(children) == 1 {
		return children[0], nil
	}
	return &Union{Children: children}, nil
}

func (p *parser) parseAnd() (Node, error) {
	first, err := p.parseDifference()
	if err != nil {
		return nil, err
	}
	children := []Node{first}
	for p.peek().kind == tokenAmp {
		p.next()
		node, err := p.parseDifference()
		if err != nil {
			return nil, err
		}
		children = append(children, node)
	}
	if len(children) == 1 {
		return children[0], nil
	}
	return &Intersection{Children: children}, nil
}

func (p *parser) parseDifference() (Node, error) {
	node, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	for p.isKeyword("but") {
		p.next()
		if err := p.expectKeyword("not"); err != nil {
			return nil, err
		}
		rhs, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		node = &Difference{A: node, B: rhs}
	}
	return node, nil
}

func (p *parser) parseTerm() (Node, error) {
	tok := p.peek()
	switch tok.kind {
	case tokenLParen:
		p.next()
		node, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokenRParen, "')'"); err != nil {
			return nil, err
		}
		return node, nil
	case tokenKeyword:
		if tok.text != "self" {
			return nil, fmt.Errorf("schema: unexpected keyword %q at offset %d", tok.text, tok.pos)
		}
		p.next()
		return &Self{}, nil
	case tokenIdent:
		p.next()
		if p.isKeyword("from") {
			p.next()
			target, err := p.expectIdent("relation after \"from\"")
			if err != nil {
				return nil, err
			}
			return &TupleToUserset{ComputedUserset: tok.text, Tupleset: target}, nil
		}
		return &RelationRef{Relation: tok.text}, nil
	default:
		return nil, fmt.Errorf("schema: expected a term at offset %d, found %q", tok.pos, tok.text)
	}
}

func (p *parser) isKeyword(word string) bool {
	tok := p.peek()
	return tok.kind == tokenKeyword && tok.text == word
}
