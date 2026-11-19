package acl

import (
	"net/netip"
	"testing"
)

func flow(proto Protocol, src, dst string, srcPort, dstPort int) Flow {
	return Flow{
		Src:      netip.MustParseAddr(src),
		Dst:      netip.MustParseAddr(dst),
		SrcPort:  srcPort,
		DstPort:  dstPort,
		Protocol: proto,
	}
}

func mustAddr(t *testing.T, s string) netip.Addr {
	t.Helper()
	addr, err := netip.ParseAddr(s)
	if err != nil {
		t.Fatalf("ParseAddr(%q): %v", s, err)
	}
	return addr
}

func registryWith(group SecurityGroup, members ...Member) *Registry {
	reg := NewRegistry()
	reg.SetGroup(group)
	for _, m := range members {
		reg.Attach(m)
	}
	return reg
}

func TestEvaluateSingleAllowRule(t *testing.T) {
	group := SecurityGroup{
		ID: "sg-app",
		Ingress: NewRuleBook(
			Rule{Number: 100, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 443, ToPort: 443, Source: CIDRSource("10.0.0.0/8")},
		),
	}
	ev := NewEvaluator(registryWith(group))

	decision, err := ev.Evaluate(Query{
		GroupID:   "sg-app",
		Direction: Ingress,
		Flow:      flow(ProtocolTCP, "10.1.2.3", "10.9.9.9", 51000, 443),
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !decision.Allowed || !decision.Matched {
		t.Fatalf("single matching allow rule was not honoured: %+v", decision)
	}
	if decision.Rule != 100 {
		t.Fatalf("matched rule %d, want 100", decision.Rule)
	}
	if decision.Stateful {
		t.Fatal("a stateless query was reported as stateful")
	}
}

func TestEvaluateSourceOutsideCIDR(t *testing.T) {
	group := SecurityGroup{
		ID: "sg-app",
		Ingress: NewRuleBook(
			Rule{Number: 100, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 443, ToPort: 443, Source: CIDRSource("10.0.0.0/8")},
		),
	}
	ev := NewEvaluator(registryWith(group))

	decision, err := ev.Evaluate(Query{
		GroupID:   "sg-app",
		Direction: Ingress,
		Flow:      flow(ProtocolTCP, "172.16.0.9", "10.9.9.9", 51000, 443),
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if decision.Allowed || decision.Matched {
		t.Fatalf("a source outside the rule CIDR was allowed: %+v", decision)
	}
}

func TestEvaluateSingleDenyRule(t *testing.T) {
	group := SecurityGroup{
		ID: "sg-app",
		Ingress: NewRuleBook(
			Rule{Number: 50, Action: ActionDeny, Protocol: ProtocolTCP, FromPort: 22, ToPort: 22, Source: CIDRSource("10.0.0.0/8")},
		),
	}
	ev := NewEvaluator(registryWith(group))

	decision, err := ev.Evaluate(Query{
		GroupID:   "sg-app",
		Direction: Ingress,
		Flow:      flow(ProtocolTCP, "10.1.2.3", "10.9.9.9", 51000, 22),
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if decision.Allowed || !decision.Matched {
		t.Fatalf("a matching deny rule was not honoured: %+v", decision)
	}
	if decision.Rule != 50 {
		t.Fatalf("matched rule %d, want 50", decision.Rule)
	}
}

func TestEvaluateImplicitDenyOnEmptyBook(t *testing.T) {
	ev := NewEvaluator(registryWith(SecurityGroup{ID: "sg-empty", Ingress: NewRuleBook()}))

	decision, err := ev.Evaluate(Query{
		GroupID:   "sg-empty",
		Direction: Ingress,
		Flow:      flow(ProtocolTCP, "10.1.2.3", "10.9.9.9", 51000, 443),
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if decision.Allowed || decision.Matched {
		t.Fatalf("an empty rule book did not deny: %+v", decision)
	}
}

func TestEvaluateEgressUsesEgressBook(t *testing.T) {
	group := SecurityGroup{
		ID: "sg-app",
		Ingress: NewRuleBook(
			Rule{Number: 100, Action: ActionDeny, Protocol: ProtocolTCP, FromPort: 443, ToPort: 443, Source: CIDRSource("10.0.0.0/8")},
		),
		Egress: NewRuleBook(
			Rule{Number: 100, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 443, ToPort: 443, Source: CIDRSource("10.0.0.0/8")},
		),
	}
	ev := NewEvaluator(registryWith(group))

	decision, err := ev.Evaluate(Query{
		GroupID:   "sg-app",
		Direction: Egress,
		Flow:      flow(ProtocolTCP, "10.1.2.3", "10.9.9.9", 51000, 443),
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !decision.Allowed || decision.Rule != 100 {
		t.Fatalf("egress book was not used: %+v", decision)
	}
}

func TestEvaluateUnknownGroup(t *testing.T) {
	ev := NewEvaluator(NewRegistry())
	if _, err := ev.Evaluate(Query{
		GroupID:   "sg-missing",
		Direction: Ingress,
		Flow:      flow(ProtocolTCP, "10.1.2.3", "10.9.9.9", 51000, 443),
	}); err == nil {
		t.Fatal("Evaluate accepted an unknown security group")
	}
}

func TestEvaluateUnknownProtocol(t *testing.T) {
	group := SecurityGroup{ID: "sg-app", Ingress: NewRuleBook()}
	ev := NewEvaluator(registryWith(group))
	if _, err := ev.Evaluate(Query{
		GroupID:   "sg-app",
		Direction: Ingress,
		Flow:      flow(Protocol("sctp"), "10.1.2.3", "10.9.9.9", 51000, 443),
	}); err == nil {
		t.Fatal("Evaluate accepted an unknown protocol")
	}
}

func TestEvaluateWithoutSource(t *testing.T) {
	ev := NewEvaluator(nil)
	if _, err := ev.Evaluate(Query{GroupID: "sg-app", Direction: Ingress}); err == nil {
		t.Fatal("Evaluate succeeded without a snapshot source")
	}
}

func TestEvaluatePrefixListSource(t *testing.T) {
	group := SecurityGroup{
		ID: "sg-app",
		Ingress: NewRuleBook(
			Rule{Number: 100, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 443, ToPort: 443, Source: PrefixListSource("corp")},
		),
	}
	reg := registryWith(group)
	if _, err := reg.SetPrefixList("corp", "10.0.0.0/8"); err != nil {
		t.Fatalf("SetPrefixList: %v", err)
	}
	ev := NewEvaluator(reg)

	decision, err := ev.Evaluate(Query{
		GroupID:   "sg-app",
		Direction: Ingress,
		Flow:      flow(ProtocolTCP, "10.4.4.4", "10.9.9.9", 51000, 443),
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !decision.Allowed || decision.Rule != 100 {
		t.Fatalf("prefix list source was not honoured: %+v", decision)
	}
}

func TestEvaluateUndefinedPrefixListFailsClosed(t *testing.T) {
	group := SecurityGroup{
		ID: "sg-app",
		Ingress: NewRuleBook(
			Rule{Number: 100, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 443, ToPort: 443, Source: PrefixListSource("absent")},
		),
	}
	ev := NewEvaluator(registryWith(group))

	decision, err := ev.Evaluate(Query{
		GroupID:   "sg-app",
		Direction: Ingress,
		Flow:      flow(ProtocolTCP, "10.4.4.4", "10.9.9.9", 51000, 443),
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if decision.Allowed {
		t.Fatal("an undefined prefix list allowed traffic")
	}
}

func TestEvaluateGroupSourceSingleMember(t *testing.T) {
	group := SecurityGroup{
		ID: "sg-db",
		Ingress: NewRuleBook(
			Rule{Number: 100, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 5432, ToPort: 5432, Source: GroupSource("sg-app")},
		),
	}
	reg := registryWith(group, member("i-app-1", "sg-app", "10.0.1.10"))

	// Traffic from the member of the referenced group is allowed.
	ev := NewEvaluator(reg)
	decision, err := ev.Evaluate(Query{
		GroupID:   "sg-db",
		Direction: Ingress,
		Flow:      flow(ProtocolTCP, "10.0.1.10", "10.0.2.20", 51000, 5432),
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("a member of the referenced group was denied: %+v", decision)
	}

	// Traffic from an unrelated address is denied by the implicit rule.
	decision, err = ev.Evaluate(Query{
		GroupID:   "sg-db",
		Direction: Ingress,
		Flow:      flow(ProtocolTCP, "10.0.1.99", "10.0.2.20", 51000, 5432),
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if decision.Allowed {
		t.Fatalf("an address outside the referenced group was allowed: %+v", decision)
	}
}

func TestEvaluateUnknownDirection(t *testing.T) {
	group := SecurityGroup{ID: "sg-app", Ingress: NewRuleBook()}
	ev := NewEvaluator(registryWith(group))
	if _, err := ev.Evaluate(Query{
		GroupID:   "sg-app",
		Direction: Direction(99),
		Flow:      flow(ProtocolTCP, "10.1.2.3", "10.9.9.9", 51000, 443),
	}); err == nil {
		t.Fatal("Evaluate accepted an unknown direction")
	}
}
