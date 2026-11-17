package match

import (
	"strings"
	"unicode"
)

// Args reports whether the arguments following argv[0] satisfy the argument
// patterns of a rule.
func Args(patterns, args []string) bool {
	if len(patterns) == 0 {
		return len(args) == 0
	}
	return glob(strings.Join(patterns, " "), strings.Join(args, " "))
}

// glob reports whether pattern matches the whole of text. A single '*' matches
// a run of non-whitespace characters; "**" matches any run, whitespace
// included. Every other character is literal.
func glob(pattern, text string) bool {
	p := []rune(pattern)
	s := []rune(text)
	memo := make([][]int8, len(p)+1)
	for i := range memo {
		memo[i] = make([]int8, len(s)+1)
	}

	var match func(i, j int) bool
	match = func(i, j int) bool {
		if state := memo[i][j]; state != 0 {
			return state == 1
		}

		ok := false
		switch {
		case i == len(p):
			ok = j == len(s)
		case i+1 < len(p) && p[i] == '*' && p[i+1] == '*':
			ok = match(i+2, j) || (j < len(s) && match(i, j+1))
		case p[i] == '*':
			ok = match(i+1, j) ||
				(j < len(s) && !unicode.IsSpace(s[j]) && match(i, j+1))
		default:
			ok = j < len(s) && p[i] == s[j] && match(i+1, j+1)
		}

		if ok {
			memo[i][j] = 1
		} else {
			memo[i][j] = 2
		}
		return ok
	}
	return match(0, 0)
}
