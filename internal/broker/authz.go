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
	decisions map[decisionCacheKey]acl.Verdict
}

type decisionCacheKey struct {
	clientID       string
	principalClass string
	action         string
	subject        string
	tenantID       string
	version        uint64
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
		decisions: make(map[decisionCacheKey]acl.Verdict),
	}
}

// Store exposes the rule store so callers can reload it.
func (a *Authorizer) Store() *acl.Store { return a.store }

// Reload installs a new policy document and invalidates cached decisions.
func (a *Authorizer) Reload(data []byte) error {
	if err := a.store.Replace(data); err != nil {
		return err
	}

	a.compiled.Purge()
	a.mu.Lock()
	a.decisions = make(map[decisionCacheKey]acl.Verdict)
	a.mu.Unlock()
	return nil
}

func connectionIdentity(conn *Connection) (clientID, principalClass string) {
	if conn == nil {
		return "", ""
	}
	return conn.ClientID, conn.PrincipalClass
}

func (a *Authorizer) cachedDecision(key decisionCacheKey) (acl.Verdict, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	v, ok := a.decisions[key]
	return v, ok
}

func (a *Authorizer) storeDecision(key decisionCacheKey, verdict acl.Verdict) {
	a.mu.Lock()
	a.decisions[key] = verdict
	a.mu.Unlock()
}

// AuthorizeSubscribe reports whether conn may subscribe to filter.
func (a *Authorizer) AuthorizeSubscribe(conn *Connection, filter string) bool {
	clientID, principalClass := connectionIdentity(conn)
	rs := a.store.Current()

	parsed, _, sharedErr := acl.ParseSharedFilter(filter)
	canonicalFilter := parsed.Filter
	tenantID := tenant.TenantOf(canonicalFilter)

	if sharedErr != nil {
		verdict := acl.Verdict{Allowed: false, Reason: sharedErr.Error()}
		a.recordSubscribe(clientID, principalClass, filter, tenant.TenantOf(filter), verdict)
		return false
	}

	key := decisionCacheKey{
		clientID:       clientID,
		principalClass: principalClass,
		action:         acl.ActionSubscribe,
		subject:        canonicalFilter,
		tenantID:       tenantID,
		version:        rs.Version,
	}
	if verdict, ok := a.cachedDecision(key); ok {
		a.recordSubscribe(clientID, principalClass, filter, tenantID, verdict)
		return verdict.Allowed
	}

	var verdict acl.Verdict
	if canonicalFilter == "" || !acl.ValidFilter(canonicalFilter) {
		verdict = acl.Verdict{Allowed: false, Reason: "invalid topic filter"}
	} else {
		compiled := a.compiled.Compile(rs, tenantID, principalClass, canonicalFilter)
		verdict = subscribeExpansionVerdict(rs, compiled)
		if coverage := acl.CheckSubscription(rs, principalClass, canonicalFilter); !coverage.Allowed {
			verdict = coverage
		}
	}

	a.storeDecision(key, verdict)
	a.recordSubscribe(clientID, principalClass, filter, tenantID, verdict)
	return verdict.Allowed
}

func subscribeExpansionVerdict(rs *acl.RuleSet, compiled acl.Compiled) acl.Verdict {
	ruleOrder := make(map[string]int)
	if rs != nil {
		for i, rule := range rs.Rules {
			ruleOrder[rule.ID] = i
		}
	}

	var uncovered acl.Verdict
	haveUncovered := false
	var denied acl.Verdict
	haveDenied := false
	deniedOrder := 0

	for _, verdict := range compiled.Verdicts {
		if verdict.Allowed {
			continue
		}
		if verdict.RuleID == "" {
			if !haveUncovered {
				uncovered = verdict
				haveUncovered = true
			}
			continue
		}

		order, known := ruleOrder[verdict.RuleID]
		if !known {
			order = len(ruleOrder)
		}
		if !haveDenied || order < deniedOrder {
			denied = verdict
			deniedOrder = order
			haveDenied = true
		}
	}

	if haveDenied {
		return denied
	}
	if haveUncovered {
		return uncovered
	}
	return acl.Verdict{Allowed: true}
}

func (a *Authorizer) recordSubscribe(clientID, principalClass, filter, tenantID string, verdict acl.Verdict) {
	ev := audit.Event{
		Action:         acl.ActionSubscribe,
		ClientID:       clientID,
		PrincipalClass: principalClass,
		Filter:         filter,
		Tenant:         tenantID,
		RuleID:         verdict.RuleID,
		Reason:         verdict.Reason,
	}
	if verdict.Allowed {
		a.audit.Allow(ev)
		return
	}
	a.audit.Deny(ev)
}

// AuthorizePublish reports whether conn may publish to topic.
func (a *Authorizer) AuthorizePublish(conn *Connection, topic string) bool {
	clientID, principalClass := connectionIdentity(conn)
	rs := a.store.Current()
	normalizedTopic := tenant.NormalizeTopic(topic)
	tenantID := tenant.TenantOf(normalizedTopic)

	key := decisionCacheKey{
		clientID:       clientID,
		principalClass: principalClass,
		action:         acl.ActionPublish,
		subject:        normalizedTopic,
		tenantID:       tenantID,
		version:        rs.Version,
	}
	if verdict, ok := a.cachedDecision(key); ok {
		a.recordPublish(clientID, principalClass, normalizedTopic, tenantID, verdict)
		return verdict.Allowed
	}

	verdict := acl.Evaluate(rs, principalClass, acl.ActionPublish, normalizedTopic)
	a.storeDecision(key, verdict)
	a.recordPublish(clientID, principalClass, normalizedTopic, tenantID, verdict)
	return verdict.Allowed
}

func (a *Authorizer) recordPublish(clientID, principalClass, topic, tenantID string, verdict acl.Verdict) {
	ev := audit.Event{
		Action:         acl.ActionPublish,
		ClientID:       clientID,
		PrincipalClass: principalClass,
		Topic:          topic,
		Tenant:         tenantID,
		RuleID:         verdict.RuleID,
		Reason:         verdict.Reason,
	}
	if verdict.Allowed {
		a.audit.Allow(ev)
		return
	}
	a.audit.Deny(ev)
}

// Subscribe asks the policy and, on approval, returns the subscription. No
// subscription state is created for a denied filter.
func (a *Authorizer) Subscribe(conn *Connection, filter string) (*Subscription, bool) {
	if !a.AuthorizeSubscribe(conn, filter) {
		return nil, false
	}
	return &Subscription{Conn: conn, Filter: filter}, true
}
