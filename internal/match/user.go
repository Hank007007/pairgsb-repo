package match

import "strings"

// User reports whether a user pattern applies to the requesting user.
// Patterns are literal user names, "*", or a group written as "%name".
func User(pattern, user string, groups []string) bool {
	switch {
	case pattern == "*":
		return true
	case strings.HasPrefix(pattern, "%"):
		group := strings.TrimPrefix(pattern, "%")
		for _, member := range groups {
			if member == group {
				return true
			}
		}
		return false
	default:
		return pattern == user
	}
}

// Host reports whether a host pattern applies to the target host. Patterns
// are literal host names or "*".
func Host(pattern, host string) bool {
	return pattern == "*" || pattern == host
}
