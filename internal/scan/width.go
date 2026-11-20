package scan

import (
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/width"
)

// Width reports the number of terminal columns occupied by r.
func Width(r rune) int {
	if isZeroWidthRune(r) {
		return 0
	}
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return 2
	default:
		return 1
	}
}

// isZeroWidthRune reports whether r renders without advancing the cursor:
// non-spacing or enclosing marks, or a formatting character the policy
// classes as invisible.
func isZeroWidthRune(r rune) bool {
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) {
		return true
	}
	return false
}

// sgrLenInText reports the byte length of a white-listed SGR sequence at the
// start of s, or 0 when s does not open with one. Input has already passed
// through the filter, so every ESC byte introduces such a sequence.
func sgrLenInText(s string) int {
	if len(s) < 3 || s[0] != ESC || s[1] != '[' {
		return 0
	}
	i := 2
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	for i < len(s) && s[i] == ';' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}
	if i < len(s) && s[i] == 'm' {
		return i + 1
	}
	return 0
}

// Cut returns the longest prefix of s that fits within limit columns.
func Cut(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	cols := 0
	i := 0
	for i < len(s) {
		if n := sgrLenInText(s[i:]); n > 0 {
			i += n
			continue
		}
		cluster, n, zwj := nextCluster(s[i:])
		if n == 0 {
			break
		}
		w := 0
		if zwj {
			w = 2
		} else {
			for _, r := range cluster {
				w += Width(r)
			}
		}
		if cols+w > limit {
			break
		}
		cols += w
		i += n
	}
	return s[:i]
}

// nextCluster returns the runes in the first grapheme cluster of s, its byte
// length, and whether the cluster is held together by a zero-width joiner.
// A cluster is a base rune followed by its attaching marks, variation
// selectors, skin-tone modifiers, and any ZWJ-joined continuations.
func nextCluster(s string) ([]rune, int, bool) {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError && size <= 1 {
		return nil, 0, false
	}
	cluster := []rune{r}
	n := size
	zwj := false
	for n < len(s) {
		next, nextSize := utf8.DecodeRuneInString(s[n:])
		if next == utf8.RuneError && nextSize <= 1 {
			break
		}
		if next == 0x200d {
			zwj = true
			cluster = append(cluster, next)
			n += nextSize
			joined, joinedSize := utf8.DecodeRuneInString(s[n:])
			if joined == utf8.RuneError && joinedSize <= 1 {
				break
			}
			cluster = append(cluster, joined)
			n += joinedSize
			continue
		}
		if !isAttachment(next) {
			break
		}
		cluster = append(cluster, next)
		n += nextSize
	}
	return cluster, n, zwj
}

// isAttachment reports whether r belongs to the cluster of the rune before
// it rather than starting a cluster of its own.
func isAttachment(r rune) bool {
	if isZeroWidthRune(r) {
		return true
	}
	switch {
	case r >= 0xfe00 && r <= 0xfe0f:
		return true
	case r >= 0x1f3fb && r <= 0x1f3ff:
		return true
	}
	return false
}
