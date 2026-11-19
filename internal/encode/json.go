package encode

import (
	"strings"

	"logline-audit-encode/internal/fields"
)

// encodeJSON renders set as a JSON object with no surrounding whitespace.
func encodeJSON(set *fields.Set) []byte {
	list := orderedFields(set)

	var b strings.Builder
	b.WriteByte('{')
	for i, f := range list {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(encodeJSONString(f.Key))
		b.WriteByte(':')
		b.WriteString(encodeJSONString(f.Value))
	}
	b.WriteByte('}')
	return []byte(b.String())
}

// encodeJSONString writes s as a quoted JSON string.
func encodeJSONString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\u2028':
			b.WriteString(`\u2028`)
		case '\u2029':
			b.WriteString(`\u2029`)
		default:
			if r < 0x20 || r == 0x7f {
				appendJSONUnicodeEscape(&b, r)
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func appendJSONUnicodeEscape(b *strings.Builder, r rune) {
	const hex = "0123456789ABCDEF"
	b.WriteString(`\u`)
	b.WriteByte(hex[(r>>12)&0x0F])
	b.WriteByte(hex[(r>>8)&0x0F])
	b.WriteByte(hex[(r>>4)&0x0F])
	b.WriteByte(hex[r&0x0F])
}
