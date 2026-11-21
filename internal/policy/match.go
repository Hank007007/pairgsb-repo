package policy

import "strings"

// Match reports the action that applies to a canonical field path. The second
// return value is false when no rule matched and the default action applies.
func (rs *RuleSet) Match(path string) (Action, bool) {
	if rs == nil {
		return defaultAction, false
	}
	concrete := splitPath(path)
	for _, rule := range rs.Rules {
		if matchPath(splitPath(rule.Path), concrete) {
			return rule.Action, true
		}
	}
	return defaultAction, false
}

func splitPath(path string) []string {
	var out []string
	var cur strings.Builder
	for _, r := range path {
		switch r {
		case '.':
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		case '[':
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			cur.WriteRune(r)
		case ']':
			cur.WriteRune(r)
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func matchPath(pattern, concrete []string) bool {
	if len(pattern) != len(concrete) {
		return false
	}
	for i, want := range pattern {
		got := concrete[i]
		if want == "[]" {
			if !isArrayIndex(got) {
				return false
			}
			continue
		}
		if !asciiEqualFold(want, got) {
			return false
		}
	}
	return true
}

func isArrayIndex(segment string) bool {
	if len(segment) < 3 || segment[0] != '[' || segment[len(segment)-1] != ']' {
		return false
	}
	for i := 1; i < len(segment)-1; i++ {
		if segment[i] < '0' || segment[i] > '9' {
			return false
		}
	}
	return true
}

func asciiEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
