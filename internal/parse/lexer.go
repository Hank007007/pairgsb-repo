package parse

import (
	"fmt"
	"strings"
)

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokName
	tokInt
	tokFloat
	tokString
	tokPunct
	tokSpread
)

type token struct {
	kind  tokenKind
	value string
	line  int
	col   int
}

func (t token) String() string {
	if t.kind == tokEOF {
		return "end of input"
	}
	return fmt.Sprintf("%q", t.value)
}

type lexer struct {
	src  string
	pos  int
	line int
	col  int
}

func newLexer(src string) *lexer {
	return &lexer{src: src, line: 1, col: 1}
}

func (l *lexer) advance(n int) {
	for i := 0; i < n && l.pos < len(l.src); i++ {
		if l.src[l.pos] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.pos++
	}
}

func (l *lexer) skipIgnored() {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',':
			l.advance(1)
		case c == '#':
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.advance(1)
			}
		case c >= 0x80:
			// Non-ASCII bytes are insignificant outside strings and comments
			// (for example a byte order mark at the very start of a request).
			l.advance(1)
		default:
			return
		}
	}
}

func isNameStart(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isNameContinue(c byte) bool {
	return isNameStart(c) || (c >= '0' && c <= '9')
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func (l *lexer) next() (token, error) {
	l.skipIgnored()
	line, col := l.line, l.col
	if l.pos >= len(l.src) {
		return token{kind: tokEOF, line: line, col: col}, nil
	}

	c := l.src[l.pos]

	if strings.HasPrefix(l.src[l.pos:], "...") {
		l.advance(3)
		return token{kind: tokSpread, value: "...", line: line, col: col}, nil
	}

	if isNameStart(c) {
		start := l.pos
		for l.pos < len(l.src) && isNameContinue(l.src[l.pos]) {
			l.advance(1)
		}
		return token{kind: tokName, value: l.src[start:l.pos], line: line, col: col}, nil
	}

	if c == '-' || isDigit(c) {
		return l.number(line, col)
	}

	if c == '"' {
		if strings.HasPrefix(l.src[l.pos:], `"""`) {
			return l.blockString(line, col)
		}
		return l.quotedString(line, col)
	}

	if strings.ContainsRune("!$&():=@[]{|}", rune(c)) {
		l.advance(1)
		return token{kind: tokPunct, value: string(c), line: line, col: col}, nil
	}

	return token{}, fmt.Errorf("line %d, column %d: unexpected character %q", line, col, string(c))
}

func (l *lexer) number(line, col int) (token, error) {
	start := l.pos
	kind := tokInt
	if l.src[l.pos] == '-' {
		l.advance(1)
	}
	if l.pos >= len(l.src) || !isDigit(l.src[l.pos]) {
		return token{}, fmt.Errorf("line %d, column %d: malformed number", line, col)
	}
	for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
		l.advance(1)
	}
	if l.pos < len(l.src) && l.src[l.pos] == '.' {
		kind = tokFloat
		l.advance(1)
		for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
			l.advance(1)
		}
	}
	if l.pos < len(l.src) && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
		kind = tokFloat
		l.advance(1)
		if l.pos < len(l.src) && (l.src[l.pos] == '+' || l.src[l.pos] == '-') {
			l.advance(1)
		}
		if l.pos >= len(l.src) || !isDigit(l.src[l.pos]) {
			return token{}, fmt.Errorf("line %d, column %d: malformed exponent", line, col)
		}
		for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
			l.advance(1)
		}
	}
	return token{kind: kind, value: l.src[start:l.pos], line: line, col: col}, nil
}

func (l *lexer) quotedString(line, col int) (token, error) {
	l.advance(1)
	var b strings.Builder
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == '"' {
			l.advance(1)
			return token{kind: tokString, value: b.String(), line: line, col: col}, nil
		}
		if c == '\n' || c == '\r' {
			return token{}, fmt.Errorf("line %d, column %d: unterminated string", line, col)
		}
		if c == '\\' {
			l.advance(1)
			if l.pos >= len(l.src) {
				break
			}
			esc := l.src[l.pos]
			switch esc {
			case '"', '\\', '/':
				b.WriteByte(esc)
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'u':
				if l.pos+4 >= len(l.src) {
					return token{}, fmt.Errorf("line %d, column %d: malformed unicode escape", line, col)
				}
				hex := l.src[l.pos+1 : l.pos+5]
				var r rune
				if _, err := fmt.Sscanf(hex, "%04x", &r); err != nil {
					return token{}, fmt.Errorf("line %d, column %d: malformed unicode escape", line, col)
				}
				b.WriteRune(r)
				l.advance(4)
			default:
				return token{}, fmt.Errorf("line %d, column %d: unknown escape \\%c", line, col, esc)
			}
			l.advance(1)
			continue
		}
		b.WriteByte(c)
		l.advance(1)
	}
	return token{}, fmt.Errorf("line %d, column %d: unterminated string", line, col)
}

func (l *lexer) blockString(line, col int) (token, error) {
	l.advance(3)
	start := l.pos
	for l.pos < len(l.src) {
		if strings.HasPrefix(l.src[l.pos:], `"""`) {
			raw := l.src[start:l.pos]
			l.advance(3)
			return token{kind: tokString, value: strings.TrimSpace(raw), line: line, col: col}, nil
		}
		l.advance(1)
	}
	return token{}, fmt.Errorf("line %d, column %d: unterminated block string", line, col)
}
