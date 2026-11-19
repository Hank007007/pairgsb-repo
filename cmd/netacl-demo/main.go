// Command netacl-demo exercises the evaluator on the scenarios from the
// incident reports so that they can be reproduced from a terminal.
//
// The demo is diagnostic tooling; it prints raw decisions and does not assert
// anything about them.
package main

import (
	"fmt"
	"net/netip"
	"os"

	"netacl/internal/acl"
)

func addr(s string) netip.Addr {
	a, err := netip.ParseAddr(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "netacl-demo: %v\n", err)
		os.Exit(2)
	}
	return a
}

func flow(proto acl.Protocol, src, dst string, srcPort, dstPort int) acl.Flow {
	return acl.Flow{Src: addr(src), Dst: addr(dst), SrcPort: srcPort, DstPort: dstPort, Protocol: proto}
}

func run(label string, ev *acl.Evaluator, q acl.Query) {
	d, err := ev.Evaluate(q)
	if err != nil {
		fmt.Printf("%-46s error: %v\n", label, err)
		return
	}
	rule := "-"
	if d.Matched {
		rule = fmt.Sprintf("%d", d.Rule)
	}
	fmt.Printf("%-46s allowed=%-5v stateful=%-5v rule=%s\n", label, d.Allowed, d.Stateful, rule)
}

func portIntervalScenario() {
	fmt.Println("== port interval ==")
	group := acl.SecurityGroup{
		ID: "sg-app",
		Ingress: acl.NewRuleBook(
			acl.Rule{Number: 100, Action: acl.ActionAllow, Protocol: acl.ProtocolTCP, FromPort: 443, ToPort: 443, Source: acl.CIDRSource("10.0.0.0/8")},
		),
	}
	reg := acl.NewRegistry()
	reg.SetGroup(group)
	ev := acl.NewEvaluator(reg)
	for _, port := range []int{22, 443, 3306} {
		run(fmt.Sprintf("tcp/%d from 10.1.2.3", port), ev, acl.Query{
			GroupID: "sg-app", Direction: acl.Ingress,
			Flow: flow(acl.ProtocolTCP, "10.1.2.3", "10.0.0.50", 51000, port),
		})
	}
}

func groupReferenceScenario() {
	fmt.Println("== group reference ==")
	group := acl.SecurityGroup{
		ID: "sg-app",
		Ingress: acl.NewRuleBook(
			acl.Rule{Number: 100, Action: acl.ActionAllow, Protocol: acl.ProtocolTCP, FromPort: 443, ToPort: 443, Source: acl.GroupSource("sg-web")},
		),
	}
	reg := acl.NewRegistry()
	reg.SetGroup(group)
	reg.Attach(acl.Member{InstanceID: "i-web-1", GroupID: "sg-web", Address: addr("10.0.1.10")})
	reg.Attach(acl.Member{InstanceID: "i-admin-1", GroupID: "sg-web-admin", Address: addr("10.0.9.9")})
	ev := acl.NewEvaluator(reg)
	run("i-web-1 (sg-web)", ev, acl.Query{
		GroupID: "sg-app", Direction: acl.Ingress,
		Flow: flow(acl.ProtocolTCP, "10.0.1.10", "10.0.0.50", 51000, 443),
	})
	run("i-admin-1 (sg-web-admin)", ev, acl.Query{
		GroupID: "sg-app", Direction: acl.Ingress,
		Flow: flow(acl.ProtocolTCP, "10.0.9.9", "10.0.0.50", 51000, 443),
	})
}

func mappedAddressScenario() {
	fmt.Println("== ipv4-mapped address ==")
	group := acl.SecurityGroup{
		ID: "sg-app",
		Ingress: acl.NewRuleBook(
			acl.Rule{Number: 80, Action: acl.ActionDeny, Protocol: acl.ProtocolTCP, FromPort: 22, ToPort: 22, Source: acl.CIDRSource("10.0.0.5/32")},
			acl.Rule{Number: 100, Action: acl.ActionAllow, Protocol: acl.ProtocolTCP, FromPort: 22, ToPort: 22, Source: acl.CIDRSource("::/0")},
		),
	}
	reg := acl.NewRegistry()
	reg.SetGroup(group)
	ev := acl.NewEvaluator(reg)
	run("10.0.0.5 -> tcp/22", ev, acl.Query{
		GroupID: "sg-app", Direction: acl.Ingress,
		Flow: flow(acl.ProtocolTCP, "10.0.0.5", "10.0.0.50", 51000, 22),
	})
	run("::ffff:10.0.0.5 -> tcp/22", ev, acl.Query{
		GroupID: "sg-app", Direction: acl.Ingress,
		Flow: flow(acl.ProtocolTCP, "::ffff:10.0.0.5", "10.0.0.50", 51000, 22),
	})
}

func orderingScenario() {
	fmt.Println("== rule ordering ==")
	group := acl.SecurityGroup{
		ID: "sg-app",
		Ingress: acl.NewRuleBook(
			acl.Rule{Number: 100, Action: acl.ActionAllow, Protocol: acl.ProtocolTCP, FromPort: 22, ToPort: 22, Source: acl.CIDRSource("10.0.0.0/8")},
			acl.Rule{Number: 90, Action: acl.ActionDeny, Protocol: acl.ProtocolTCP, FromPort: 22, ToPort: 22, Source: acl.CIDRSource("10.0.0.0/8")},
		),
	}
	reg := acl.NewRegistry()
	reg.SetGroup(group)
	ev := acl.NewEvaluator(reg)
	for i := 1; i <= 5; i++ {
		run(fmt.Sprintf("10.0.0.5 -> tcp/22 (attempt %d)", i), ev, acl.Query{
			GroupID: "sg-app", Direction: acl.Ingress,
			Flow: flow(acl.ProtocolTCP, "10.0.0.5", "10.0.0.50", 51000, 22),
		})
	}
}

func statefulScenario() {
	fmt.Println("== stateful return traffic ==")
	group := acl.SecurityGroup{
		ID: "sg-db",
		Ingress: acl.NewRuleBook(
			acl.Rule{Number: 100, Action: acl.ActionAllow, Protocol: acl.ProtocolTCP, FromPort: 5432, ToPort: 5432, Source: acl.CIDRSource("10.0.1.10/32")},
		),
	}
	reg := acl.NewRegistry()
	reg.SetGroup(group)
	ev := acl.NewEvaluator(reg)

	established := flow(acl.ProtocolTCP, "10.0.1.10", "10.0.2.20", 51000, 5432)
	ev.Connections().Establish(established)
	run("reply 10.0.2.20:5432 -> 10.0.1.10:51000", ev, acl.Query{
		GroupID: "sg-db", Direction: acl.Ingress,
		Flow: flow(acl.ProtocolTCP, "10.0.2.20", "10.0.1.10", 5432, 51000),
	})
	run("same-direction 10.0.1.10:52000 -> 10.0.2.20:22", ev, acl.Query{
		GroupID: "sg-db", Direction: acl.Ingress,
		Flow: flow(acl.ProtocolTCP, "10.0.1.10", "10.0.2.20", 52000, 22),
	})
	run("udp 10.0.1.10:52000 -> 10.0.2.20:53", ev, acl.Query{
		GroupID: "sg-db", Direction: acl.Ingress,
		Flow: flow(acl.ProtocolUDP, "10.0.1.10", "10.0.2.20", 52000, 53),
	})
	run("forged 10.0.2.20:51000 -> 10.0.1.10:5432", ev, acl.Query{
		GroupID: "sg-db", Direction: acl.Ingress,
		Flow: flow(acl.ProtocolTCP, "10.0.2.20", "10.0.1.10", 51000, 5432),
	})

	// Policy tightens: the rule that admitted the connection is withdrawn.
	tightened := acl.SecurityGroup{
		ID:      "sg-db",
		Ingress: acl.NewRuleBook(),
	}
	reg.SetGroup(tightened)
	run("reply after tightening", ev, acl.Query{
		GroupID: "sg-db", Direction: acl.Ingress,
		Flow: flow(acl.ProtocolTCP, "10.0.2.20", "10.0.1.10", 5432, 51000),
	})
}

func main() {
	portIntervalScenario()
	fmt.Println()
	groupReferenceScenario()
	fmt.Println()
	mappedAddressScenario()
	fmt.Println()
	orderingScenario()
	fmt.Println()
	statefulScenario()
}
