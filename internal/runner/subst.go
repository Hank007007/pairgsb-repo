package runner

import (
	"fmt"
	"strings"
)

// expandPlaceholders substitutes ${INPUT_name} references in tmpl with the
// matching value from inputs.
//
// A reference is ${INPUT_ followed by one or more [A-Za-z0-9_] characters and
// a closing brace. Anything that does not match that shape is kept as literal
// text.
//
// Substitution is a single left-to-right pass: a value that is written into
// the output is never scanned again, so ${INPUT_other} or ${ sequences inside
// a value survive verbatim (policy §2.1).
func expandPlaceholders(tmpl string, inputs map[string]string) (string, error) {
	if !strings.Contains(tmpl, "${") {
		return tmpl, nil
	}
	var b strings.Builder
	b.Grow(len(tmpl))
	for i := 0; i < len(tmpl); {
		name, width, ok := placeholderAt(tmpl, i)
		if !ok {
			b.WriteByte(tmpl[i])
			i++
			continue
		}
		value, exists := inputs[name]
		if !exists {
			return "", fmt.Errorf("%w: ${INPUT_%s}", ErrUnknownPlaceholder, name)
		}
		b.WriteString(value)
		i += width
	}
	return b.String(), nil
}

// placeholderAt reports whether a placeholder starts at offset i. It returns
// the input name and the number of bytes the reference spans.
func placeholderAt(s string, i int) (name string, width int, ok bool) {
	const prefix = "${INPUT_"
	if !strings.HasPrefix(s[i:], prefix) {
		return "", 0, false
	}
	j := i + len(prefix)
	start := j
	for j < len(s) && isNameByte(s[j]) {
		j++
	}
	if j == start || j >= len(s) || s[j] != '}' {
		return "", 0, false
	}
	return s[start:j], j + 1 - i, true
}

func isNameByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z':
		return true
	case c >= 'A' && c <= 'Z':
		return true
	case c >= '0' && c <= '9':
		return true
	case c == '_':
		return true
	}
	return false
}

