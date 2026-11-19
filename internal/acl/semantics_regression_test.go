package acl

import "testing"

func TestSemanticsPortIntervalsAreClosed(t *testing.T) {
	group := SecurityGroup{
		ID: "sg-app",
		Ingress: NewRuleBook(
			Rule{Number: 100, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 443, ToPort: 443, Source: CIDRSource("10.0.0.0/8")},
			Rule{Number: 200, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 8000, ToPort: 8080, Source: CIDRSource("10.0.0.0/8")},
			Rule{Number: 300, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 9000, ToPort: 9100, Source: CIDRSource("10.0.0.0/8")},
		),
	}
	ev := NewEvaluator(registryWith(group))

	cases := []struct {
		port    int
		allowed bool
		rule    RuleNumber
	}{
		{22, false, 0},
		{443, true, 100},
		{3306, false, 0},
		{6379, false, 0},
		{7999, false, 0},
		{8000, true, 200},
		{8042, true, 200},
		{8080, true, 200},
		{8081, false, 0},
		{8999, false, 0},
		{9000, true, 300},
		{9050, true, 300},
		{9100, true, 300},
		{9101, false, 0},
	}
	for _, tc := range cases {
		decision, err := ev.Evaluate(Query{
			GroupID:   "sg-app",
			Direction: Ingress,
			Flow:      flow(ProtocolTCP, "10.1.2.3", "10.0.0.50", 51000, tc.port),
		})
		if err != nil {
			t.Fatalf("tcp/%d: Evaluate: %v", tc.port, err)
		}
		if decision.Allowed != tc.allowed || decision.Rule != tc.rule {
			t.Fatalf("tcp/%d: got allowed=%v rule=%d, want allowed=%v rule=%d",
				tc.port, decision.Allowed, decision.Rule, tc.allowed, tc.rule)
		}
	}
}

func TestSemanticsGroupReferenceUsesExactID(t *testing.T) {
	group := SecurityGroup{
		ID: "sg-app",
		Ingress: NewRuleBook(
			Rule{Number: 100, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 443, ToPort: 443, Source: GroupSource("sg-web")},
		),
	}
	reg := registryWith(group,
		member("i-web-1", "sg-web", "10.0.1.10"),
		member("i-admin-1", "sg-web-admin", "10.0.9.9"),
	)
	ev := NewEvaluator(reg)

	web, err := ev.Evaluate(Query{
		GroupID:   "sg-app",
		Direction: Ingress,
		Flow:      flow(ProtocolTCP, "10.0.1.10", "10.0.0.50", 51000, 443),
	})
	if err != nil {
		t.Fatalf("Evaluate sg-web member: %v", err)
	}
	if !web.Allowed || web.Rule != 100 {
		t.Fatalf("sg-web member was not admitted by rule 100: %+v", web)
	}

	admin, err := ev.Evaluate(Query{
		GroupID:   "sg-app",
		Direction: Ingress,
		Flow:      flow(ProtocolTCP, "10.0.9.9", "10.0.0.50", 51000, 443),
	})
	if err != nil {
		t.Fatalf("Evaluate sg-web-admin member: %v", err)
	}
	if admin.Allowed || admin.Matched {
		t.Fatalf("sg-web-admin member was resolved as sg-web: %+v", admin)
	}
}

func TestSemanticsIPv4MappedSourceIsNormalizedBeforeMatching(t *testing.T) {
	group := SecurityGroup{
		ID: "sg-app",
		Ingress: NewRuleBook(
			Rule{Number: 80, Action: ActionDeny, Protocol: ProtocolTCP, FromPort: 22, ToPort: 22, Source: CIDRSource("10.0.0.5/32")},
			Rule{Number: 100, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 22, ToPort: 22, Source: CIDRSource("::/0")},
		),
	}
	ev := NewEvaluator(registryWith(group))

	for _, src := range []string{"10.0.0.5", "::ffff:10.0.0.5"} {
		decision, err := ev.Evaluate(Query{
			GroupID:   "sg-app",
			Direction: Ingress,
			Flow:      flow(ProtocolTCP, src, "10.0.0.50", 51000, 22),
		})
		if err != nil {
			t.Fatalf("Evaluate %s: %v", src, err)
		}
		if decision.Allowed || decision.Rule != 80 {
			t.Fatalf("%s bypassed the explicit deny: %+v", src, decision)
		}
	}

	nativeIPv6, err := ev.Evaluate(Query{
		GroupID:   "sg-app",
		Direction: Ingress,
		Flow:      flow(ProtocolTCP, "2001:db8::1", "10.0.0.50", 51000, 22),
	})
	if err != nil {
		t.Fatalf("Evaluate native IPv6: %v", err)
	}
	if !nativeIPv6.Allowed || nativeIPv6.Rule != 100 {
		t.Fatalf("native IPv6 source did not match ::/0: %+v", nativeIPv6)
	}
}

func TestSemanticsPrefixListNormalizesMappedSource(t *testing.T) {
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
		Flow:      flow(ProtocolTCP, "::ffff:10.4.4.4", "10.0.0.50", 51000, 443),
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !decision.Allowed || decision.Rule != 100 {
		t.Fatalf("mapped address did not match the prefix list: %+v", decision)
	}
}

func TestSemanticsRuleOrderIsNumericAndStable(t *testing.T) {
	ninetyWins := SecurityGroup{
		ID: "sg-app",
		Ingress: NewRuleBook(
			Rule{Number: 100, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 22, ToPort: 22, Source: CIDRSource("10.0.0.0/8")},
			Rule{Number: 90, Action: ActionDeny, Protocol: ProtocolTCP, FromPort: 22, ToPort: 22, Source: CIDRSource("10.0.0.0/8")},
		),
	}
	ev := NewEvaluator(registryWith(ninetyWins))
	for i := 0; i < 200; i++ {
		decision, err := ev.Evaluate(Query{
			GroupID:   "sg-app",
			Direction: Ingress,
			Flow:      flow(ProtocolTCP, "10.0.0.5", "10.0.0.50", 51000, 22),
		})
		if err != nil {
			t.Fatalf("Evaluate attempt %d: %v", i+1, err)
		}
		if decision.Allowed || decision.Rule != 90 {
			t.Fatalf("attempt %d: got allowed=%v rule=%d, want deny rule 90",
				i+1, decision.Allowed, decision.Rule)
		}
	}

	numericOrder := SecurityGroup{
		ID: "sg-app",
		Ingress: NewRuleBook(
			Rule{Number: 10, Action: ActionDeny, Protocol: ProtocolTCP, FromPort: 443, ToPort: 443, Source: CIDRSource("10.0.0.0/8")},
			Rule{Number: 2, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 443, ToPort: 443, Source: CIDRSource("10.0.0.0/8")},
		),
	}
	ev = NewEvaluator(registryWith(numericOrder))
	for i := 0; i < 200; i++ {
		decision, err := ev.Evaluate(Query{
			GroupID:   "sg-app",
			Direction: Ingress,
			Flow:      flow(ProtocolTCP, "10.0.0.5", "10.0.0.50", 51000, 443),
		})
		if err != nil {
			t.Fatalf("Evaluate attempt %d: %v", i+1, err)
		}
		if !decision.Allowed || decision.Rule != 2 {
			t.Fatalf("attempt %d: got allowed=%v rule=%d, want allow rule 2",
				i+1, decision.Allowed, decision.Rule)
		}
	}
}

func TestSemanticsStatefulRequiresFullReversedFiveTuple(t *testing.T) {
	group := SecurityGroup{
		ID: "sg-db",
		Ingress: NewRuleBook(
			Rule{Number: 100, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 5432, ToPort: 5432, Source: CIDRSource("10.0.1.10/32")},
		),
	}
	reg := registryWith(group)
	ev := NewEvaluator(reg)

	initiated := flow(ProtocolTCP, "10.0.1.10", "10.0.2.20", 51000, 5432)
	ev.Connections().Establish(initiated)

	reply, err := ev.Evaluate(Query{
		GroupID:   "sg-db",
		Direction: Ingress,
		Flow:      flow(ProtocolTCP, "10.0.2.20", "10.0.1.10", 5432, 51000),
	})
	if err != nil {
		t.Fatalf("Evaluate reply: %v", err)
	}
	if !reply.Allowed || !reply.Stateful || reply.Matched {
		t.Fatalf("valid return traffic was not admitted statefully: %+v", reply)
	}

	notReplies := []struct {
		name string
		pkt  Flow
	}{
		{"same direction", flow(ProtocolTCP, "10.0.1.10", "10.0.2.20", 51000, 5432)},
		{"same addresses, ports not reversed", flow(ProtocolTCP, "10.0.2.20", "10.0.1.10", 51000, 5432)},
		{"reversed addresses, protocol differs", flow(ProtocolUDP, "10.0.2.20", "10.0.1.10", 5432, 51000)},
		{"new connection on another port", flow(ProtocolTCP, "10.0.1.10", "10.0.2.20", 52000, 22)},
	}
	for _, tc := range notReplies {
		decision, err := ev.Evaluate(Query{
			GroupID:   "sg-db",
			Direction: Ingress,
			Flow:      tc.pkt,
		})
		if err != nil {
			t.Fatalf("%s: Evaluate: %v", tc.name, err)
		}
		if decision.Stateful {
			t.Fatalf("%s: incorrectly treated as return traffic: %+v", tc.name, decision)
		}
	}

	reg.SetGroup(SecurityGroup{ID: "sg-db", Ingress: NewRuleBook()})
	afterTightening, err := ev.Evaluate(Query{
		GroupID:   "sg-db",
		Direction: Ingress,
		Flow:      flow(ProtocolTCP, "10.0.2.20", "10.0.1.10", 5432, 51000),
	})
	if err != nil {
		t.Fatalf("Evaluate reply after tightening: %v", err)
	}
	if !afterTightening.Allowed || !afterTightening.Stateful {
		t.Fatalf("established return traffic was killed by a tightening policy: %+v", afterTightening)
	}

	if _, err := ev.Evaluate(Query{
		GroupID:   "sg-db",
		Direction: Direction(99),
		Flow:      flow(ProtocolTCP, "10.0.2.20", "10.0.1.10", 5432, 51000),
	}); err == nil {
		t.Fatal("an established flow bypassed direction validation")
	}
}
