package parse

import (
	"fmt"
	"strings"

	"gqladmit/internal/graphql"
)

// Parser turns GraphQL request text into a [graphql.Document].
type Parser struct {
	lex *lexer
	tok token
}

// Parse parses a GraphQL document. It returns an error when the text is not
// syntactically valid.
func Parse(src string) (*graphql.Document, error) {
	p := &Parser{lex: newLexer(src)}
	if err := p.read(); err != nil {
		return nil, err
	}

	doc := &graphql.Document{Fragments: map[string]*graphql.Fragment{}}

	for p.tok.kind != tokEOF {
		switch {
		case p.tok.kind == tokPunct && p.tok.value == "{":
			sel, err := p.parseSelectionSet()
			if err != nil {
				return nil, err
			}
			doc.Operations = append(doc.Operations, &graphql.Operation{
				Type:         graphql.Query,
				SelectionSet: sel,
			})
		case p.tok.kind == tokName && (p.tok.value == "query" || p.tok.value == "mutation" || p.tok.value == "subscription"):
			op, err := p.parseOperation()
			if err != nil {
				return nil, err
			}
			doc.Operations = append(doc.Operations, op)
		case p.tok.kind == tokName && p.tok.value == "fragment":
			frag, err := p.parseFragmentDefinition()
			if err != nil {
				return nil, err
			}
			if _, dup := doc.Fragments[frag.Name]; dup {
				return nil, fmt.Errorf("line %d, column %d: duplicate fragment %q", p.tok.line, p.tok.col, frag.Name)
			}
			doc.Fragments[frag.Name] = frag
		default:
			return nil, fmt.Errorf("line %d, column %d: unexpected %s at top level", p.tok.line, p.tok.col, p.tok)
		}
	}

	if len(doc.Operations) == 0 {
		return nil, fmt.Errorf("document contains no operations")
	}

	return doc, nil
}

func (p *Parser) read() error {
	tok, err := p.lex.next()
	if err != nil {
		return err
	}
	p.tok = tok
	return nil
}

func (p *Parser) expectPunct(v string) error {
	if p.tok.kind != tokPunct || p.tok.value != v {
		return fmt.Errorf("line %d, column %d: expected %q, found %s", p.tok.line, p.tok.col, v, p.tok)
	}
	return p.read()
}

func (p *Parser) expectName() (string, error) {
	if p.tok.kind != tokName {
		return "", fmt.Errorf("line %d, column %d: expected a name, found %s", p.tok.line, p.tok.col, p.tok)
	}
	name := p.tok.value
	return name, p.read()
}

func (p *Parser) parseOperation() (*graphql.Operation, error) {
	op := &graphql.Operation{Type: graphql.OperationType(p.tok.value)}
	if err := p.read(); err != nil {
		return nil, err
	}

	if p.tok.kind == tokName {
		op.Name = p.tok.value
		if err := p.read(); err != nil {
			return nil, err
		}
	}

	if p.tok.kind == tokPunct && p.tok.value == "(" {
		vars, err := p.parseVariableDefinitions()
		if err != nil {
			return nil, err
		}
		op.Variables = vars
	}

	dirs, err := p.parseDirectives()
	if err != nil {
		return nil, err
	}
	op.Directives = dirs

	sel, err := p.parseSelectionSet()
	if err != nil {
		return nil, err
	}
	op.SelectionSet = sel
	return op, nil
}

func (p *Parser) parseVariableDefinitions() ([]*graphql.VariableDefinition, error) {
	if err := p.expectPunct("("); err != nil {
		return nil, err
	}
	var defs []*graphql.VariableDefinition
	for !(p.tok.kind == tokPunct && p.tok.value == ")") {
		if p.tok.kind != tokPunct || p.tok.value != "$" {
			return nil, fmt.Errorf("line %d, column %d: expected \"$\", found %s", p.tok.line, p.tok.col, p.tok)
		}
		if err := p.read(); err != nil {
			return nil, err
		}
		name, err := p.expectName()
		if err != nil {
			return nil, err
		}
		if err := p.expectPunct(":"); err != nil {
			return nil, err
		}
		typ, err := p.parseTypeRef()
		if err != nil {
			return nil, err
		}
		def := &graphql.VariableDefinition{Name: name, Type: typ}
		if p.tok.kind == tokPunct && p.tok.value == "=" {
			if err := p.read(); err != nil {
				return nil, err
			}
			val, err := p.parseValue(true)
			if err != nil {
				return nil, err
			}
			def.Default = val
		}
		if _, err := p.parseDirectives(); err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	return defs, p.read()
}

func (p *Parser) parseTypeRef() (string, error) {
	var b strings.Builder
	if p.tok.kind == tokPunct && p.tok.value == "[" {
		if err := p.read(); err != nil {
			return "", err
		}
		inner, err := p.parseTypeRef()
		if err != nil {
			return "", err
		}
		b.WriteString("[" + inner + "]")
		if err := p.expectPunct("]"); err != nil {
			return "", err
		}
	} else {
		name, err := p.expectName()
		if err != nil {
			return "", err
		}
		b.WriteString(name)
	}
	if p.tok.kind == tokPunct && p.tok.value == "!" {
		b.WriteString("!")
		if err := p.read(); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

func (p *Parser) parseFragmentDefinition() (*graphql.Fragment, error) {
	if err := p.read(); err != nil { // consume "fragment"
		return nil, err
	}
	if p.tok.kind == tokName && p.tok.value == "on" {
		return nil, fmt.Errorf("line %d, column %d: fragment name must not be \"on\"", p.tok.line, p.tok.col)
	}
	name, err := p.expectName()
	if err != nil {
		return nil, err
	}
	if p.tok.kind != tokName || p.tok.value != "on" {
		return nil, fmt.Errorf("line %d, column %d: expected \"on\", found %s", p.tok.line, p.tok.col, p.tok)
	}
	if err := p.read(); err != nil {
		return nil, err
	}
	typeCond, err := p.expectName()
	if err != nil {
		return nil, err
	}
	dirs, err := p.parseDirectives()
	if err != nil {
		return nil, err
	}
	sel, err := p.parseSelectionSet()
	if err != nil {
		return nil, err
	}
	return &graphql.Fragment{Name: name, TypeCondition: typeCond, Directives: dirs, SelectionSet: sel}, nil
}

func (p *Parser) parseSelectionSet() ([]graphql.Selection, error) {
	if err := p.expectPunct("{"); err != nil {
		return nil, err
	}
	var sels []graphql.Selection
	for !(p.tok.kind == tokPunct && p.tok.value == "}") {
		if p.tok.kind == tokEOF {
			return nil, fmt.Errorf("line %d, column %d: unterminated selection set", p.tok.line, p.tok.col)
		}
		sel, err := p.parseSelection()
		if err != nil {
			return nil, err
		}
		sels = append(sels, sel)
	}
	if err := p.read(); err != nil { // consume "}"
		return nil, err
	}
	return sels, nil
}

func (p *Parser) parseSelection() (graphql.Selection, error) {
	if p.tok.kind == tokSpread {
		return p.parseFragmentSelection()
	}

	name, err := p.expectName()
	if err != nil {
		return nil, err
	}

	field := &graphql.Field{Name: name}

	if p.tok.kind == tokPunct && p.tok.value == ":" {
		if err := p.read(); err != nil {
			return nil, err
		}
		real, err := p.expectName()
		if err != nil {
			return nil, err
		}
		field.Alias = name
		field.Name = real
	}

	if p.tok.kind == tokPunct && p.tok.value == "(" {
		args, err := p.parseArguments()
		if err != nil {
			return nil, err
		}
		field.Arguments = args
	}

	dirs, err := p.parseDirectives()
	if err != nil {
		return nil, err
	}
	field.Directives = dirs

	if p.tok.kind == tokPunct && p.tok.value == "{" {
		sel, err := p.parseSelectionSet()
		if err != nil {
			return nil, err
		}
		field.SelectionSet = sel
	}

	return field, nil
}

func (p *Parser) parseFragmentSelection() (graphql.Selection, error) {
	if err := p.read(); err != nil { // consume "..."
		return nil, err
	}

	if p.tok.kind == tokName && p.tok.value == "on" {
		if err := p.read(); err != nil {
			return nil, err
		}
		typeCond, err := p.expectName()
		if err != nil {
			return nil, err
		}
		dirs, err := p.parseDirectives()
		if err != nil {
			return nil, err
		}
		sel, err := p.parseSelectionSet()
		if err != nil {
			return nil, err
		}
		return &graphql.InlineFragment{TypeCondition: typeCond, Directives: dirs, SelectionSet: sel}, nil
	}

	dirs, err := p.parseDirectives()
	if err != nil {
		return nil, err
	}

	if p.tok.kind == tokPunct && p.tok.value == "{" {
		sel, err := p.parseSelectionSet()
		if err != nil {
			return nil, err
		}
		return &graphql.InlineFragment{Directives: dirs, SelectionSet: sel}, nil
	}

	name, err := p.expectName()
	if err != nil {
		return nil, err
	}
	return &graphql.FragmentSpread{Name: name, Directives: dirs}, nil
}

func (p *Parser) parseArguments() ([]*graphql.Argument, error) {
	if err := p.expectPunct("("); err != nil {
		return nil, err
	}
	var args []*graphql.Argument
	for !(p.tok.kind == tokPunct && p.tok.value == ")") {
		name, err := p.expectName()
		if err != nil {
			return nil, err
		}
		if err := p.expectPunct(":"); err != nil {
			return nil, err
		}
		val, err := p.parseValue(false)
		if err != nil {
			return nil, err
		}
		args = append(args, &graphql.Argument{Name: name, Value: val})
	}
	return args, p.read()
}

func (p *Parser) parseDirectives() ([]*graphql.Directive, error) {
	var dirs []*graphql.Directive
	for p.tok.kind == tokPunct && p.tok.value == "@" {
		if err := p.read(); err != nil {
			return nil, err
		}
		name, err := p.expectName()
		if err != nil {
			return nil, err
		}
		dir := &graphql.Directive{Name: name}
		if p.tok.kind == tokPunct && p.tok.value == "(" {
			args, err := p.parseArguments()
			if err != nil {
				return nil, err
			}
			dir.Arguments = args
		}
		dirs = append(dirs, dir)
	}
	return dirs, nil
}

func (p *Parser) parseValue(isConst bool) (graphql.Value, error) {
	if p.tok.kind == tokPunct && p.tok.value == "$" {
		if isConst {
			return nil, fmt.Errorf("line %d, column %d: variable is not allowed here", p.tok.line, p.tok.col)
		}
		if err := p.read(); err != nil {
			return nil, err
		}
		name, err := p.expectName()
		if err != nil {
			return nil, err
		}
		return &graphql.Variable{Name: name}, nil
	}

	switch p.tok.kind {
	case tokInt:
		v := &graphql.IntValue{Raw: p.tok.value}
		return v, p.read()
	case tokFloat:
		v := &graphql.FloatValue{Raw: p.tok.value}
		return v, p.read()
	case tokString:
		v := &graphql.StringValue{Value: p.tok.value}
		return v, p.read()
	case tokName:
		switch p.tok.value {
		case "true", "false":
			v := &graphql.BooleanValue{Value: p.tok.value == "true"}
			return v, p.read()
		case "null":
			v := &graphql.NullValue{}
			return v, p.read()
		default:
			v := &graphql.EnumValue{Value: p.tok.value}
			return v, p.read()
		}
	case tokPunct:
		switch p.tok.value {
		case "[":
			if err := p.read(); err != nil {
				return nil, err
			}
			list := &graphql.ListValue{}
			for !(p.tok.kind == tokPunct && p.tok.value == "]") {
				elem, err := p.parseValue(isConst)
				if err != nil {
					return nil, err
				}
				list.Values = append(list.Values, elem)
			}
			return list, p.read()
		case "{":
			if err := p.read(); err != nil {
				return nil, err
			}
			obj := &graphql.ObjectValue{}
			for !(p.tok.kind == tokPunct && p.tok.value == "}") {
				name, err := p.expectName()
				if err != nil {
					return nil, err
				}
				if err := p.expectPunct(":"); err != nil {
					return nil, err
				}
				val, err := p.parseValue(isConst)
				if err != nil {
					return nil, err
				}
				obj.Fields = append(obj.Fields, &graphql.ObjectField{Name: name, Value: val})
			}
			return obj, p.read()
		}
	}

	return nil, fmt.Errorf("line %d, column %d: expected a value, found %s", p.tok.line, p.tok.col, p.tok)
}
