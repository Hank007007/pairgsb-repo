package shape

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// safePunct lists the non-alphanumeric ASCII characters admitted by ClassSafe.
const safePunct = "-_.:@/ "

// CheckClass verifies that s belongs to the requested character class and
// returns the value in canonical form.
func CheckClass(s string, class Class) (string, error) {
	if !classAllowed(s, class) {
		return "", ErrCharset
	}
	return norm.NFKC.String(s), nil
}

func classAllowed(s string, class Class) bool {
	switch class {
	case ClassAny:
		return true
	case ClassText:
		for _, r := range s {
			if !unicode.IsGraphic(r) {
				return false
			}
		}
		return true
	case ClassSafe:
		for _, r := range s {
			if r < 0x80 {
				if !safeASCII(byte(r)) {
					return false
				}
				continue
			}
			if unicode.IsControl(r) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func safeASCII(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	case b >= '0' && b <= '9':
		return true
	}
	return strings.IndexByte(safePunct, b) >= 0
}
