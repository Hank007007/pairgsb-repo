package policy

import "strings"

// Match reports the action that applies to a canonical field path. The second
// return value is false when no rule matched and the default action applies.
func (rs *RuleSet) Match(path string) (Action, bool) {
	if rs == nil {
		return defaultAction, false
	}
	for _, rule := range rs.Rules {
		if strings.HasPrefix(path, rule.Path) {
			return rule.Action, true
		}
	}
	return defaultAction, false
}
