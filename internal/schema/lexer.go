package schema

import (
	"fmt"
	"unicode"
)

type tokenKind int

const (
	tokenEOF tokenKind = iota
	tokenIdent
	tokenKeyword
	tokenLBrace
	tokenRBrace
	tokenLParen
	tokenRParen
	tokenPipe
	tokenAmp
	tokenEquals
)

type token struct {
	kind tokenKind
	text string
	pos  int
}

var keywords = map[string]bool{
	"type":     true,
	"relation": true,
	"self":     true,
	"from":     true,
	"but":      true,
	"not":      true,
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentPart(r rune) bool {
	return r == '_' || r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func tokenize(input string) ([]token, error) {
	var tokens []token
	runes := []rune(input)
	i := 0
	for i < len(runes) {
		r := runes[i]

		switch {
		case unicode.IsSpace(r):
			i++
			continue
		case r == '/' && i+1 < len(runes) && runes[i+1] == '/':
			for i < len(runes) && runes[i] != '\n' {
				i++
			}
			continue
		case r == '/' && i+1 < len(runes) && runes[i+1] == '*':
			start := i
			i += 2
			closed := false
			for i+1 < len(runes) {
				if runes[i] == '*' && runes[i+1] == '/' {
					i += 2
					closed = true
					break
				}
				i++
			}
			if !closed {
				return nil, fmt.Errorf("schema: unterminated comment at offset %d", start)
			}
			continue
		}

		pos := i
		switch r {
		case '{':
			tokens = append(tokens, token{kind: tokenLBrace, text: "{", pos: pos})
			i++
			continue
		case '}':
			tokens = append(tokens, token{kind: tokenRBrace, text: "}", pos: pos})
			i++
			continue
		case '(':
			tokens = append(tokens, token{kind: tokenLParen, text: "(", pos: pos})
			i++
			continue
		case ')':
			tokens = append(tokens, token{kind: tokenRParen, text: ")", pos: pos})
			i++
			continue
		case '|':
			tokens = append(tokens, token{kind: tokenPipe, text: "|", pos: pos})
			i++
			continue
		case '&':
			tokens = append(tokens, token{kind: tokenAmp, text: "&", pos: pos})
			i++
			continue
		case '=':
			tokens = append(tokens, token{kind: tokenEquals, text: "=", pos: pos})
			i++
			continue
		}

		if !isIdentStart(r) {
			return nil, fmt.Errorf("schema: unexpected character %q at offset %d", r, pos)
		}
		start := i
		i++
		for i < len(runes) && isIdentPart(runes[i]) {
			i++
		}
		text := string(runes[start:i])
		kind := tokenIdent
		if keywords[text] {
			kind = tokenKeyword
		}
		tokens = append(tokens, token{kind: kind, text: text, pos: pos})
	}

	tokens = append(tokens, token{kind: tokenEOF, pos: len(runes)})
	return tokens, nil
}
