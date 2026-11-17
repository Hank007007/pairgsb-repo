// Package lexer turns rule source text into a token stream.
package lexer

import (
	"strconv"

	"rulelang/internal/limits"
	"rulelang/internal/rlerr"
)

// Kind enumerates token categories.
type Kind int

const (
	EOF Kind = iota
	Ident
	String
	Int
	CIDR
	LParen
	RParen
	And
	Or
	Not
	In
	Eq
	Ne
	Lt
	Le
	Gt
	Ge
)

// String renders a token kind for diagnostics.
func (k Kind) String() string {
	switch k {
	case EOF:
		return "end of input"
	case Ident:
		return "identifier"
	case String:
		return "string"
	case Int:
		return "integer"
	case CIDR:
		return "cidr"
	case LParen:
		return "("
	case RParen:
		return ")"
	case And:
		return "and"
	case Or:
		return "or"
	case Not:
		return "not"
	case In:
		return "in"
	case Eq:
		return "=="
	case Ne:
		return "!="
	case Lt:
		return "<"
	case Le:
		return "<="
	case Gt:
		return ">"
	case Ge:
		return ">="
	}
	return "unknown"
}

// Token is one lexical unit. Text holds the decoded value for strings and the
// literal spelling for everything else. IntVal is set for Int tokens.
type Token struct {
	Kind   Kind
	Text   string
	IntVal int64
	Pos    int
}

// Lex scans src and returns the token stream, terminated by an EOF token.
func Lex(src string) ([]Token, error) {
	if len(src) > limits.MaxSourceBytes {
		return nil, rlerr.New(rlerr.StageLimit, 0, "rule source is too long")
	}
	var toks []Token
	i := 0
	for i < len(src) {
		c := src[i]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			i++
			continue
		}
		start := i
		switch {
		case isIdentStart(c):
			j := i + 1
			for j < len(src) && isIdentPart(src[j]) {
				j++
			}
			word := src[i:j]
			kind := Ident
			switch word {
			case "and":
				kind = And
			case "or":
				kind = Or
			case "not":
				kind = Not
			case "in":
				kind = In
			}
			toks = append(toks, Token{Kind: kind, Text: word, Pos: start})
			i = j
		case c >= '0' && c <= '9':
			tok, next, err := scanNumber(src, i)
			if err != nil {
				return nil, err
			}
			toks = append(toks, tok)
			i = next
		case c == '"':
			text, next, err := scanString(src, i)
			if err != nil {
				return nil, err
			}
			toks = append(toks, Token{Kind: String, Text: text, Pos: start})
			i = next
		case c == '(':
			toks = append(toks, Token{Kind: LParen, Text: "(", Pos: start})
			i++
		case c == ')':
			toks = append(toks, Token{Kind: RParen, Text: ")", Pos: start})
			i++
		case c == '=':
			if i+1 < len(src) && src[i+1] == '=' {
				toks = append(toks, Token{Kind: Eq, Text: "==", Pos: start})
				i += 2
			} else {
				return nil, rlerr.New(rlerr.StageLex, start, "expected '=='")
			}
		case c == '!':
			if i+1 < len(src) && src[i+1] == '=' {
				toks = append(toks, Token{Kind: Ne, Text: "!=", Pos: start})
				i += 2
			} else {
				return nil, rlerr.New(rlerr.StageLex, start, "expected '!='")
			}
		case c == '<':
			if i+1 < len(src) && src[i+1] == '=' {
				toks = append(toks, Token{Kind: Le, Text: "<=", Pos: start})
				i += 2
			} else {
				toks = append(toks, Token{Kind: Lt, Text: "<", Pos: start})
				i++
			}
		case c == '>':
			if i+1 < len(src) && src[i+1] == '=' {
				toks = append(toks, Token{Kind: Ge, Text: ">=", Pos: start})
				i += 2
			} else {
				toks = append(toks, Token{Kind: Gt, Text: ">", Pos: start})
				i++
			}
		default:
			return nil, rlerr.New(rlerr.StageLex, start, "unexpected character "+quoteByte(c))
		}
		if len(toks) > limits.MaxTokens {
			return nil, rlerr.New(rlerr.StageLimit, start, "too many tokens")
		}
	}
	toks = append(toks, Token{Kind: EOF, Pos: len(src)})
	return toks, nil
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9') || c == '.'
}

// scanString reads a double-quoted string starting at src[start]. It returns
// the decoded contents and the index just past the closing quote.
func scanString(src string, start int) (string, int, error) {
	var out []byte
	i := start + 1
	for {
		if i >= len(src) {
			return "", 0, rlerr.New(rlerr.StageLex, start, "unterminated string")
		}
		c := src[i]
		if c == '"' {
			return string(out), i + 1, nil
		}
		if c == '\n' {
			return "", 0, rlerr.New(rlerr.StageLex, i, "newline in string")
		}
		if c == '\\' {
			if i+1 >= len(src) {
				out = append(out, c)
				i++
				continue
			}
			esc := src[i+1]
			switch esc {
			case '\\':
				out = append(out, '\\')
			case '"':
				out = append(out, '"')
			case 'n':
				out = append(out, '\n')
			case 't':
				out = append(out, '\t')
			default:
				out = append(out, esc)
			}
			i += 2
			continue
		}
		out = append(out, c)
		i++
	}
}

// scanNumber reads a decimal integer, or an IPv4 CIDR when the digits are
// followed by a dotted-quad and a prefix length.
func scanNumber(src string, start int) (Token, int, error) {
	i := start
	for i < len(src) && src[i] >= '0' && src[i] <= '9' {
		i++
	}
	if i < len(src) && src[i] == '.' {
		end, err := scanCIDRTail(src, i)
		if err != nil {
			return Token{}, 0, err
		}
		return Token{Kind: CIDR, Text: src[start:end], Pos: start}, end, nil
	}
	digits := src[start:i]
	if i < len(src) && isIdentStart(src[i]) {
		return Token{}, 0, rlerr.New(rlerr.StageLex, i, "identifier cannot start with a digit")
	}
	v := parseInt64(digits)
	return Token{Kind: Int, Text: digits, IntVal: v, Pos: start}, i, nil
}

// scanCIDRTail validates the ".d.d.d/len" part beginning at the first dot.
func scanCIDRTail(src string, dot int) (int, error) {
	i := dot
	for octet := 0; octet < 3; octet++ {
		if i >= len(src) || src[i] != '.' {
			return 0, rlerr.New(rlerr.StageLex, i, "malformed cidr")
		}
		i++
		n := 0
		for i < len(src) && src[i] >= '0' && src[i] <= '9' {
			n = n*10 + int(src[i]-'0')
			i++
			if n > 255 {
				return 0, rlerr.New(rlerr.StageLex, i, "cidr octet out of range")
			}
		}
		if n > 255 {
			return 0, rlerr.New(rlerr.StageLex, i, "cidr octet out of range")
		}
	}
	if i >= len(src) || src[i] != '/' {
		return 0, rlerr.New(rlerr.StageLex, i, "malformed cidr")
	}
	i++
	bits := -1
	for i < len(src) && src[i] >= '0' && src[i] <= '9' {
		if bits < 0 {
			bits = 0
		}
		bits = bits*10 + int(src[i]-'0')
		i++
	}
	if bits < 0 || bits > 32 {
		return 0, rlerr.New(rlerr.StageLex, i, "cidr prefix out of range")
	}
	return i, nil
}

func quoteByte(c byte) string {
	if c >= 0x20 && c < 0x7f {
		return "'" + string(rune(c)) + "'"
	}
	const hex = "0123456789abcdef"
	return "'\\x" + string([]byte{hex[c>>4], hex[c&0x0f]}) + "'"
}

// parseInt64 converts a run of decimal digits to an integer value.
func parseInt64(digits string) int64 {
	v, _ := strconv.Atoi(digits)
	return int64(v)
}
