// Package broker wires the ACL to the three places an MQTT broker asks for an
// authorization decision.
package broker

import (
	"sync"

	"topicwarden/internal/acl"
	"topicwarden/internal/audit"
	"topicwarden/internal/tenant"
)

// Connection is one authenticated client session. The principal class belongs
// to the session and is replaced when the client re-authenticates.
type Connection struct {
	ClientID       string
	PrincipalClass string
}

// Authorizer answers subscribe, publish, and delivery questions.
type Authorizer struct {
	store    *acl.Store
	compiled *acl.CompiledCache
	audit    *audit.Logger

	mu        sync.Mutex
	decisions map[string]acl.Verdict
	grants    map[string]acl.Verdict
}

// NewAuthorizer returns an Authorizer backed by store.
func NewAuthorizer(store *acl.Store, log *audit.Logger) *Authorizer {
	if store == nil {
		store = acl.NewStore(nil)
	}
	return &Authorizer{
		store:     store,
		compiled:  acl.NewCompiledCache(),
		audit:     log,
		decisions: make(map[string]acl.Verdict),
		grants:    make(map[string]acl.Verdict),
	}
}

// Store exposes the rule store so callers can reload it.
func (a *Authorizer) Store() *acl.Store { return a.store }

// Reload installs a new policy document.
func (a *Authorizer) Reload(data []byte) error {
	return a.store.Replace(data)
}

func decisionKey(conn *Connection, action, subject string) string {
	return conn.ClientID + "|" + subject
}

// AuthorizeSubscribe reports whether conn may subscribe to filter.
func (a *Authorizer) AuthorizeSubscribe(conn *Connection, filter string) bool {
	tenantID := tenant.TenantOf(filter)

	a.mu.Lock()
	if v, ok := a.decisions[decisionKey(conn, acl.ActionSubscribe, filter)]; ok {
		a.mu.Unlock()
		a.recordSubscribe(conn, filter, v)
		return v.Allowed
	}
	a.mu.Unlock()

	compiled := a.compiled.Compile(a.store.Current(), tenantID, conn.PrincipalClass, filter)

	verdict := acl.Verdict{Allowed: false, Reason: "no matching rule"}
	for _, v := range compiled.Verdicts {
		if v.Allowed {
			verdict = v
			break
		}
	}

	a.mu.Lock()
	a.decisions[decisionKey(conn, acl.ActionSubscribe, filter)] = verdict
	a.mu.Unlock()

	a.recordSubscribe(conn, filter, verdict)
	return verdict.Allowed
}

func (a *Authorizer) recordSubscribe(conn *Connection, filter string, v acl.Verdict) {
	ev := audit.Event{
		Action:         acl.ActionSubscribe,
		ClientID:       conn.ClientID,
		PrincipalClass: conn.PrincipalClass,
		Filter:         filter,
		Tenant:         tenant.TenantOf(filter),
		RuleID:         v.RuleID,
		Reason:         v.Reason,
	}
	if v.Allowed {
		a.audit.Allow(ev)
		return
	}
	a.audit.Deny(ev)
}

// AuthorizePublish reports whether conn may publish to topic.
func (a *Authorizer) AuthorizePublish(conn *Connection, topic string) bool {
	a.mu.Lock()
	if v, ok := a.decisions[decisionKey(conn, acl.ActionPublish, topic)]; ok {
		a.mu.Unlock()
		a.recordPublish(conn, topic, v)
		return v.Allowed
	}
	a.mu.Unlock()

	rule, ok := acl.RuleMatches(a.store.Current(), conn.PrincipalClass, acl.ActionPublish, topic)
	verdict := acl.Verdict{Allowed: false, Reason: "no matching rule"}
	if ok {
		verdict = acl.Verdict{
			Allowed: rule.Effect == acl.EffectAllow,
			RuleID:  rule.ID,
		}
	}

	a.mu.Lock()
	a.decisions[decisionKey(conn, acl.ActionPublish, topic)] = verdict
	a.mu.Unlock()

	a.recordPublish(conn, topic, verdict)
	return verdict.Allowed
}

func (a *Authorizer) recordPublish(conn *Connection, topic string, v acl.Verdict) {
	ev := audit.Event{
		Action:         acl.ActionPublish,
		ClientID:       conn.ClientID,
		PrincipalClass: conn.PrincipalClass,
		Topic:          topic,
		Tenant:         tenant.TenantOf(topic),
		RuleID:         v.RuleID,
		Reason:         v.Reason,
	}
	if v.Allowed {
		a.audit.Allow(ev)
		return
	}
	a.audit.Deny(ev)
}

// Subscribe asks the policy and, on approval, records the subscription.
func (a *Authorizer) Subscribe(conn *Connection, filter string) (*Subscription, bool) {
	if !a.AuthorizeSubscribe(conn, filter) {
		return nil, false
	}
	sub := &Subscription{Conn: conn, Filter: filter}
	a.mu.Lock()
	a.grants[decisionKey(conn, acl.ActionSubscribe, filter)] = acl.Verdict{Allowed: true}
	a.mu.Unlock()
	return sub, true
}
