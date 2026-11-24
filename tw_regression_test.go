package topicwarden_test

import (
	"strings"
	"testing"

	"topicwarden/internal/acl"
	"topicwarden/internal/audit"
	"topicwarden/internal/broker"
	"topicwarden/internal/tenant"
)

func TestTWRegressionMatchingTable(t *testing.T) {
	cases := []struct {
		filter string
		topic  string
		want   bool
	}{
		{"sensors/#", "sensors", true},
		{"sensors/#", "sensors/a", true},
		{"sensors/#", "sensors/a/b", true},
		{"sensors/#", "sensorsx", false},
		{"#", "anything/here", true},
		{"a/+/c", "a/b/c", true},
		{"a/+/c", "a//c", false},
		{"a/+/c", "a/b/c/d", false},
		{"a/+", "a/", false},
		{"+", "a", true},
		{"a/b", "/a/b", false},
		{"a/b", "/a/b/", false},
		{"/a/b/", "/a/b/", true},
		{"a/b", "a//b", false},
		{"a/b", "a%2Fb", true},
		{"a/b", "a%2fb", true},
		{"a/#/b", "a/b", false},
	}
	for _, tc := range cases {
		if got := acl.Match(tc.filter, tc.topic); got != tc.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tc.filter, tc.topic, got, tc.want)
		}
	}
}

func TestTWRegressionTenantCaseAndEncoding(t *testing.T) {
	if got := tenant.NormalizeTopic("tenant%2FACME%2Fsensors%2Fa%2Ftemp"); got != "tenant/ACME/sensors/a/temp" {
		t.Fatalf("normalized = %q", got)
	}
	if got := tenant.TenantOf("tenant/ACME/x"); got != "ACME" {
		t.Fatalf("ACME tenant = %q", got)
	}
	if tenant.TenantOf("tenant/acme/x") == tenant.TenantOf("tenant/ACME/x") {
		t.Fatal("tenant ids must be case-sensitive")
	}
}

func TestTWRegressionUnknownEffectKeepsOldPolicy(t *testing.T) {
	store := acl.NewStore(mustLoadTW(t, `
rules:
  - id: old
    effect: allow
    principal: sensor
    action: pub
    filter: old/#
`))
	a := broker.NewAuthorizer(store, audit.NewLogger(nil))
	if err := a.Reload([]byte(`
rules:
  - id: bad
    effect: maybe
    principal: sensor
    action: pub
    filter: new/#
`)); err == nil {
		t.Fatal("unknown effect must fail load")
	}
	if !a.AuthorizePublish(&broker.Connection{ClientID: "c", PrincipalClass: "sensor"}, "old/x") {
		t.Fatal("old policy must remain active after failed reload")
	}
}

func TestTWRegressionDenyPrecedenceAndTenant(t *testing.T) {
	policy := `
rules:
  - id: deny-first
    effect: deny
    principal: sensor
    action: pub
    filter: tenant/acme/sensors/#
    tenant: acme
  - id: allow-second
    effect: allow
    principal: sensor
    action: pub
    filter: tenant/acme/sensors/#
    tenant: acme
`
	rs := mustLoadTW(t, policy)
	v := acl.Evaluate(rs, "sensor", acl.ActionPublish, "tenant/acme/sensors/a")
	if v.Allowed || v.RuleID != "deny-first" {
		t.Fatalf("unexpected verdict: %+v", v)
	}

	rs2 := mustLoadTW(t, `
rules:
  - id: allow-second
    effect: allow
    principal: sensor
    action: pub
    filter: tenant/acme/sensors/#
    tenant: acme
  - id: deny-second
    effect: deny
    principal: sensor
    action: pub
    filter: tenant/acme/sensors/#
    tenant: acme
`)
	v2 := acl.Evaluate(rs2, "sensor", acl.ActionPublish, "tenant/acme/sensors/a")
	if v2.Allowed || v2.RuleID != "deny-second" {
		t.Fatalf("order must not matter: %+v", v2)
	}

	other := acl.Evaluate(rs2, "sensor", acl.ActionPublish, "tenant/globex/sensors/a")
	if other.Allowed || other.RuleID != "" {
		t.Fatalf("tenant-scoped rule must not apply elsewhere: %+v", other)
	}
}

func TestTWRegressionSubscribeCoverageAuditAndShare(t *testing.T) {
	log := audit.NewLogger(nil)
	a := newTWAuthorizer(t, log, `
rules:
  - id: temp
    effect: allow
    principal: sensor
    action: sub
    filter: sensors/+/temp
`)
	conn := &broker.Connection{ClientID: "c-1", PrincipalClass: "sensor"}
	if a.AuthorizeSubscribe(conn, "sensors/+/temp/#") {
		t.Fatal("partial # coverage must be denied")
	}
	events := log.Events()
	if len(events) != 1 || events[0].Decision != audit.DecisionDeny {
		t.Fatalf("events = %+v, want one deny", events)
	}
	if events[0].Filter != "sensors/+/temp/#" {
		t.Fatalf("audit filter = %q", events[0].Filter)
	}

	if !a.AuthorizeSubscribe(conn, "$share/g/sensors/+/temp") {
		t.Fatal("valid shared subscription must be allowed")
	}
	if a.AuthorizeSubscribe(conn, "$share/g/sensors/+/+") {
		t.Fatal("shared subscription must require full inner coverage")
	}
	if a.AuthorizeSubscribe(conn, "$share//sensors/+/temp") {
		t.Fatal("empty shared group must be denied")
	}
	if a.AuthorizeSubscribe(conn, "$share/g/") {
		t.Fatal("empty shared filter must be denied")
	}
}

func TestTWRegressionEncodedTopicParityAndTenant(t *testing.T) {
	log := audit.NewLogger(nil)
	a := newTWAuthorizer(t, log, `
rules:
  - id: acme-sub
    effect: allow
    principal: sensor
    action: sub
    filter: tenant/acme/sensors/+/temp
    tenant: acme
  - id: acme-pub
    effect: allow
    principal: sensor
    action: pub
    filter: tenant/acme/sensors/+/temp
    tenant: acme
`)
	conn := &broker.Connection{ClientID: "c", PrincipalClass: "sensor"}
	encoded := "tenant%2Facme%2Fsensors%2Fa%2Ftemp"
	if !a.AuthorizeSubscribe(conn, encoded) {
		t.Fatal("encoded subscribe must normalize like publish")
	}
	if !a.AuthorizePublish(conn, encoded) {
		t.Fatal("encoded publish must normalize like subscribe")
	}
	if a.AuthorizePublish(conn, "tenant/globex/sensors/a/temp") {
		t.Fatal("tenant rule must not cross tenants")
	}
	for _, ev := range log.Events() {
		if strings.Contains(ev.Topic, "%2") {
			t.Fatalf("encoded topic was not normalized in audit: %q", ev.Topic)
		}
	}
}

func TestTWRegressionReloadReauthAndDelivery(t *testing.T) {
	log := audit.NewLogger(nil)
	a := newTWAuthorizer(t, log, `
rules:
  - id: allow-temp
    effect: allow
    principal: sensor
    action: sub
    filter: sensors/+/temp
`)
	conn := &broker.Connection{ClientID: "c", PrincipalClass: "sensor"}
	sub, ok := a.Subscribe(conn, "$share/g/sensors/+/temp")
	if !ok {
		t.Fatal("shared subscribe failed")
	}
	if !a.Deliver(sub, "sensors/a/temp") {
		t.Fatal("matching shared delivery should be allowed")
	}

	if err := a.Reload([]byte(`
rules:
  - id: deny-temp
    effect: deny
    principal: sensor
    action: sub
    filter: sensors/+/temp
`)); err != nil {
		t.Fatal(err)
	}
	if a.AuthorizeSubscribe(conn, "sensors/a/temp") {
		t.Fatal("cached allow must not survive reload")
	}
	if a.Deliver(sub, "sensors/a/temp") {
		t.Fatal("existing subscription must obey new deny")
	}

	if err := a.Reload([]byte(`
rules:
  - id: anon
    effect: allow
    principal: anonymous
    action: sub
    filter: sensors/+/temp
`)); err != nil {
		t.Fatal(err)
	}
	conn.PrincipalClass = "anonymous"
	if !a.AuthorizeSubscribe(conn, "sensors/a/temp") {
		t.Fatal("new anonymous class should use its own rules")
	}
	conn.PrincipalClass = "sensor"
	if a.AuthorizeSubscribe(conn, "sensors/a/temp") {
		t.Fatal("old sensor class cache must not authorize after switching back under anon-only policy")
	}
}

func TestTWRegressionNoDeliveryAuditForNonCandidate(t *testing.T) {
	log := audit.NewLogger(nil)
	a := newTWAuthorizer(t, log, `
rules:
  - id: allow
    effect: allow
    principal: sensor
    action: sub
    filter: sensors/+/temp
`)
	conn := &broker.Connection{ClientID: "c", PrincipalClass: "sensor"}
	sub, ok := a.Subscribe(conn, "sensors/+/temp")
	if !ok {
		t.Fatal("subscribe failed")
	}
	before := len(log.Events())
	if a.Deliver(sub, "other/a/hum") {
		t.Fatal("nonmatching topic delivered")
	}
	if len(log.Events()) != before {
		t.Fatal("non-candidate delivery must not write audit")
	}
}

func TestTWRegressionHashSubscriptionDeliversBaseTopic(t *testing.T) {
	a := newTWAuthorizer(t, audit.NewLogger(nil), `
rules:
  - id: sensors-all
    effect: allow
    principal: sensor
    action: sub
    filter: sensors/#
`)
	conn := &broker.Connection{ClientID: "c", PrincipalClass: "sensor"}
	sub, ok := a.Subscribe(conn, "sensors/#")
	if !ok {
		t.Fatal("sensors/# should be subscribable under sensors/# allow")
	}
	if !a.Deliver(sub, "sensors") {
		t.Fatal("sensors/# must deliver to base sensors topic")
	}
	if !a.Deliver(sub, "sensors/a/b") {
		t.Fatal("sensors/# must deliver to nested topics")
	}
}

func TestTWRegressionHashSubscriptionWithHashRule(t *testing.T) {
	a := newTWAuthorizer(t, audit.NewLogger(nil), `
rules:
  - id: all
    effect: allow
    principal: operator
    action: sub
    filter: "#"
`)
	conn := &broker.Connection{ClientID: "ops", PrincipalClass: "operator"}
	if !a.AuthorizeSubscribe(conn, "#") {
		t.Fatal("a # rule must cover a # subscription")
	}
}

func TestTWRegressionExactCoverageCounterexamples(t *testing.T) {
	rs := mustLoadTW(t, `
rules:
  - id: only-x
    effect: allow
    principal: sensor
    action: sub
    filter: a/x
`)
	if v := acl.CheckSubscription(rs, "sensor", "a/+"); v.Allowed {
		t.Fatal("a/x must not cover every a/+ expansion")
	}
	brokerAuthz := newTWAuthorizer(t, audit.NewLogger(nil), `
rules:
  - id: only-x
    effect: allow
    principal: sensor
    action: sub
    filter: a/x
`)
	if brokerAuthz.AuthorizeSubscribe(&broker.Connection{ClientID: "c", PrincipalClass: "sensor"}, "a/+") {
		t.Fatal("broker must reject expansion missed by fixed probes")
	}

	rs = mustLoadTW(t, `
rules:
  - id: root
    effect: allow
    principal: sensor
    action: sub
    filter: a
  - id: one
    effect: allow
    principal: sensor
    action: sub
    filter: a/+
  - id: two
    effect: allow
    principal: sensor
    action: sub
    filter: a/+/+
  - id: three
    effect: allow
    principal: sensor
    action: sub
    filter: a/+/+/+
`)
	if v := acl.CheckSubscription(rs, "sensor", "a/#"); v.Allowed {
		t.Fatal("finite wildcard rules must not cover arbitrary # depth")
	}

	rs = mustLoadTW(t, `
rules:
  - id: acme
    effect: allow
    principal: sensor
    action: sub
    filter: tenant/acme/sensors/#
    tenant: acme
`)
	if v := acl.CheckSubscription(rs, "sensor", "tenant/+/sensors/#"); v.Allowed {
		t.Fatal("one tenant rule must not cover wildcard tenant expansion")
	}
	if v := acl.CheckSubscription(rs, "sensor", "tenant/acme/sensors/#"); !v.Allowed {
		t.Fatalf("same-tenant # filter should be covered: %+v", v)
	}
}

func TestTWRegressionWildcardDenyAuditsFirstIntersectingRule(t *testing.T) {
	log := audit.NewLogger(nil)
	a := newTWAuthorizer(t, log, `
rules:
  - id: earlier-literal-deny
    effect: deny
    principal: sensor
    action: sub
    filter: a/y
  - id: later-wildcard-deny
    effect: deny
    principal: sensor
    action: sub
    filter: a/+
  - id: wildcard-allow
    effect: allow
    principal: sensor
    action: sub
    filter: a/+
`)
	conn := &broker.Connection{ClientID: "c", PrincipalClass: "sensor"}
	if a.AuthorizeSubscribe(conn, "a/+") {
		t.Fatal("intersecting deny must reject subscription")
	}
	events := log.Events()
	if len(events) != 1 || events[0].RuleID != "earlier-literal-deny" {
		t.Fatalf("events = %+v, want earlier-literal-deny", events)
	}
}

func mustLoadTW(t *testing.T, yaml string) *acl.RuleSet {
	t.Helper()
	rs, err := acl.LoadRules([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

func newTWAuthorizer(t *testing.T, log *audit.Logger, yaml string) *broker.Authorizer {
	t.Helper()
	a := broker.NewAuthorizer(nil, log)
	if err := a.Reload([]byte(yaml)); err != nil {
		t.Fatal(err)
	}
	return a
}
