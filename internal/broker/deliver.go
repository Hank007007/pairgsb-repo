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
// sub.
func (a *Authorizer) Deliver(sub *Subscription, topic string) bool {
	if sub == nil || sub.Conn == nil {
		return false
	}

	a.mu.Lock()
	verdict, ok := a.grants[decisionKey(sub.Conn, acl.ActionSubscribe, sub.Filter)]
	a.mu.Unlock()

	if !ok {
		return false
	}
	if !acl.Match(sub.Filter, topic) {
		return false
	}

	ev := audit.Event{
		Action:         acl.ActionSubscribe,
		ClientID:       sub.Conn.ClientID,
		PrincipalClass: sub.Conn.PrincipalClass,
		Topic:          topic,
		Tenant:         tenant.TenantOf(topic),
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
