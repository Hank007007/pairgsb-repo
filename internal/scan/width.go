package scan

import (
	"unicode"
	"unicode/utf8"
)

// Width reports the number of terminal columns occupied by r.
func Width(r rune) int {
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
		return 0
	}
	return 1
}

// Cut returns the longest prefix of s that fits within limit columns.
func Cut(s string, limit int) string {
	cols := 0
	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		w := Width(r)
		if w == 0 {
			i += size
			continue
		}
		if cols+w > limit {
			break
		}
		cols += w
		i += w
	}
	return s[:i]
}
