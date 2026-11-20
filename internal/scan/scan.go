// Package scan recognises escape sequences and decodes individual runes
// inside untrusted log text.
package scan

import "unicode/utf8"

// ASCII bytes referenced by the scanner.
const (
	ESC = 0x1b
	BEL = 0x07
)

// Kind identifies the shape of a recognised sequence.
type Kind int

const (
	// KindCSI is a control sequence introducer, ESC [ ... final.
	KindCSI Kind = iota + 1
	// KindOSC is an operating system command, ESC ] ... terminator.
	KindOSC
	// KindDCS is a device control string, ESC P ... terminator.
	KindDCS
)

// Sequence is one escape sequence located at the start of the input.
type Sequence struct {
	Kind Kind
	Raw  []byte
}

// Next inspects the start of data and reports the escape sequence found
// there. The second result is false when data does not start with a complete
// sequence.
func Next(data []byte) (Sequence, bool) {
	if len(data) < 2 || data[0] != ESC {
		return Sequence{}, false
	}
	if data[1] != '[' {
		return Sequence{}, false
	}
	i := 2
	for i < len(data) && data[i] >= '0' && data[i] <= '9' {
		i++
	}
	if i < len(data) && data[i] == 'm' {
		return Sequence{Kind: KindCSI, Raw: data[:i+1]}, true
	}
	return Sequence{}, false
}

// Class groups decoded runes by the way the renderer must treat them.
type Class int

const (
	// Text is an ordinary printable rune.
	Text Class = iota
	// C0 is a C0 control character.
	C0
	// C1 is a C1 control character.
	C1
	// Bidi is a bidirectional formatting character.
	Bidi
	// ZeroWidth is an invisible formatting character.
	ZeroWidth
)

// Decoded is one rune taken from the input together with its byte length and
// the class the renderer should use when writing it.
type Decoded struct {
	Rune  rune
	Size  int
	Class Class
}

// DecodeRune decodes the first rune of data.
func DecodeRune(data []byte) Decoded {
	r, size := utf8.DecodeRune(data)
	class := Text
	switch {
	case r < 0x20 || r == 0x7f:
		class = C0
	case isZeroWidth(r):
		class = ZeroWidth
	}
	return Decoded{Rune: r, Size: size, Class: class}
}

// isZeroWidth reports whether r is an invisible formatting character that
// carries no glyph of its own.
func isZeroWidth(r rune) bool {
	switch r {
	case 0x200b, 0x200e, 0x200f, 0xfeff:
		return true
	}
	return r >= 0x2060 && r <= 0x2064
}

// Visible renders the bytes of a sequence as printable text, spelling the
// non-printing bytes as backslash escapes.
func Visible(raw []byte) string {
	out := make([]byte, 0, len(raw)+8)
	for _, c := range raw {
		switch {
		case c == ESC:
			out = append(out, `\x1b`...)
		case c < 0x20 || c == 0x7f:
			out = append(out, Token(rune(c))...)
		default:
			out = append(out, c)
		}
	}
	return string(out)
}

// Token spells a single control rune as a printable backslash escape.
func Token(r rune) string {
	const hex = "0123456789abcdef"
	if r < 0x80 {
		return string([]byte{'\\', 'x', hex[(r>>4)&0xf], hex[r&0xf]})
	}
	buf := []byte{'\\', 'u', '0', '0', '0', '0'}
	for i := 5; i >= 2; i-- {
		buf[i] = hex[r&0xf]
		r >>= 4
	}
	return string(buf)
}
