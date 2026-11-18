package acl

// SelectRule returns the rule that governs access of uid to target, together
// with the pattern that selected it. The boolean is false when no rule of that
// uid speaks about target at all, in which case access is refused.
func (p *Policy) SelectRule(uid int, target string) (*Rule, string, bool) {
	for pattern, rule := range p.byUID[uid] {
		if matchPath(pattern, target) {
			return rule, pattern, true
		}
	}
	return nil, "", false
}
