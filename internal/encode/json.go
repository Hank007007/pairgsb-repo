package encode

import (
	"fmt"
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
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return fmt.Sprintf(`"%s"`, s)
}
