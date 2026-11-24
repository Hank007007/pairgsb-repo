package broker

import (
	"topicwarden/internal/acl"
	"topicwarden/internal/audit"
	"topicwarden/internal/tenant"
)

// Subscription is a live subscription held by a connection.
type Subscription struct {
	Conn   *Connection
	Filter string
}

// Deliver reports whether a message published on topic should be handed to
// sub. Matching uses the inner filter of a shared subscription, while
// authorization is always evaluated against the current principal and policy
// version.
func (a *Authorizer) Deliver(sub *Subscription, topic string) bool {
	if sub == nil || sub.Conn == nil || a == nil {
		return false
	}

	normalizedTopic := tenant.NormalizeTopic(topic)
	parsed, _, err := acl.ParseSharedFilter(sub.Filter)
	if err != nil {
		return false
	}
	if !acl.Match(parsed.Filter, normalizedTopic) {
		return false
	}

	principalClass := sub.Conn.PrincipalClass
	verdict := acl.Evaluate(a.store.Current(), principalClass, acl.ActionSubscribe, normalizedTopic)

	ev := audit.Event{
		Action:         acl.ActionSubscribe,
		ClientID:       sub.Conn.ClientID,
		PrincipalClass: principalClass,
		Topic:          normalizedTopic,
		Tenant:         tenant.TenantOf(normalizedTopic),
		RuleID:         verdict.RuleID,
		Reason:         verdict.Reason,
	}
	if verdict.Allowed {
		a.audit.Allow(ev)
		return true
	}
	a.audit.Deny(ev)
	return false
}
