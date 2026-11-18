package policy

import "strings"

// Allow reports whether rule applies to a call from peer to method.
func Allow(peer, method string, rule Rule) bool {
	if !matchFrom(peer, rule.From) {
		return false
	}
	return matchSegments(rule.To, method)
}

func matchFrom(peer, from string) bool {
	if from == "*" {
		return true
	}
	if from == "" {
		return false
	}
	return strings.HasPrefix(peer, from)
}

// matchSegments compares a slash-separated pattern against a value. A "*"
// pattern segment stands for any one non-empty value segment.
func matchSegments(pattern, value string) bool {
	if pattern == value {
		return true
	}
	p := segments(pattern)
	v := segments(value)
	if len(p) != len(v) {
		return false
	}
	for i := range p {
		if p[i] == "*" {
			if v[i] == "" {
				return false
			}
			continue
		}
		if p[i] != v[i] {
			return false
		}
	}
	return true
}

func segments(s string) []string {
	s = strings.TrimPrefix(s, "/")
	if s == "" {
		return nil
	}
	return strings.Split(s, "/")
}
