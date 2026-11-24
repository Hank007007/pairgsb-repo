package acl

import "topicwarden/internal/tenant"

// Action values a rule may declare.
const (
	ActionSubscribe = "sub"
	ActionPublish   = "pub"
)

// Decision is the normalized, version-bound context for one authorization
// evaluation.
type Decision struct {
	PrincipalClass string
	Action         string
	Topic          string
	Tenant         string
	Version        uint64
}

// Verdict is the outcome of evaluating one concrete topic.
type Verdict struct {
	Allowed bool
	RuleID  string
	Reason  string
}

// EvaluateDecision evaluates a concrete decision context. The topic is
// normalized and its tenant derived by the shared evaluation path.
func EvaluateDecision(rs *RuleSet, decision Decision) Verdict {
	return Evaluate(rs, decision.PrincipalClass, decision.Action, decision.Topic)
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
//
// Deny rules take precedence over allow rules. Among multiple matching deny
// rules, the first rule in document order is returned.
func RuleMatches(rs *RuleSet, principalClass, action, topic string) (Rule, bool) {
	if rs == nil {
		return Rule{}, false
	}

	normalized := tenant.NormalizeTopic(topic)
	tenantID := tenant.TenantOf(normalized)

	var firstAllow Rule
	sawAllow := false
	for _, rule := range rs.Rules {
		if rule.Principal != principalClass || rule.Action != action {
			continue
		}
		if !RuleAppliesToTenant(rule, tenantID) {
			continue
		}
		if !Match(rule.Filter, normalized) {
			continue
		}

		switch rule.Effect {
		case EffectDeny:
			return rule, true
		case EffectAllow:
			if !sawAllow {
				firstAllow = rule
				sawAllow = true
			}
		}
	}

	if sawAllow {
		return firstAllow, true
	}
	return Rule{}, false
}

// Evaluate decides one concrete topic for principalClass and action under rs.
func Evaluate(rs *RuleSet, principalClass, action, topic string) Verdict {
	normalized := tenant.NormalizeTopic(topic)
	if normalized == "" {
		return Verdict{Allowed: false, Reason: "empty topic"}
	}

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
