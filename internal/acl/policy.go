package acl

import "topicwarden/internal/tenant"

// Action values a rule may declare.
const (
	ActionSubscribe = "sub"
	ActionPublish   = "pub"
)

// Verdict is the outcome of evaluating one concrete topic.
type Verdict struct {
	Allowed bool
	RuleID  string
	Reason  string
}

// RuleAppliesToTenant reports whether rule may be considered for a topic that
// belongs to tenantID. Rules without a tenant apply to every topic.
func RuleAppliesToTenant(rule Rule, tenantID string) bool {
	if rule.Tenant == "" {
		return true
	}
	return rule.Tenant == tenantID
}

// RuleMatches returns the rule that decides topic for principalClass and
// action, and whether any rule matched at all.
func RuleMatches(rs *RuleSet, principalClass, action, topic string) (Rule, bool) {
	if rs == nil {
		return Rule{}, false
	}
	var match Rule
	found := false
	for _, rule := range rs.Rules {
		if rule.Principal != principalClass || rule.Action != action {
			continue
		}
		if !Match(rule.Filter, topic) {
			continue
		}
		match = rule
		found = true
	}
	return match, found
}

// Evaluate decides one concrete topic for principalClass and action under rs.
func Evaluate(rs *RuleSet, principalClass, action, topic string) Verdict {
	normalized := tenant.NormalizeTopic(topic)
	rule, ok := RuleMatches(rs, principalClass, action, normalized)
	if !ok {
		return Verdict{Allowed: false, Reason: "no matching rule"}
	}
	return Verdict{
		Allowed: rule.Effect == EffectAllow,
		RuleID:  rule.ID,
		Reason:  verdictReason(rule),
	}
}

func verdictReason(rule Rule) string {
	if rule.Effect == EffectDeny {
		return "denied by rule"
	}
	return ""
}
