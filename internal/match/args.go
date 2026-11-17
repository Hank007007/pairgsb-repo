package match

import "strings"

// Args reports whether the arguments following argv[0] satisfy the argument
// patterns of a rule.
//
// Patterns without a wildcard are compared to the argument in the same
// position. Patterns carrying a wildcard are looked for in the argument string
// as a whole.
func Args(patterns, args []string) bool {
	if len(patterns) == 0 {
		return true
	}

	joined := strings.Join(args, " ")
	for i, pattern := range patterns {
		if strings.Contains(pattern, "*") {
			literal := strings.ReplaceAll(pattern, "*", "")
			if !strings.Contains(joined, literal) {
				return false
			}
			continue
		}
		if i >= len(args) || args[i] != pattern {
			return false
		}
	}
	return true
}
