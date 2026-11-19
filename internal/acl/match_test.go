package acl

import "testing"

func TestProtocolMatchesExact(t *testing.T) {
	if !ProtocolMatches(ProtocolTCP, ProtocolTCP) {
		t.Fatal("tcp rule did not match tcp traffic")
	}
	if ProtocolMatches(ProtocolTCP, ProtocolUDP) {
		t.Fatal("tcp rule matched udp traffic")
	}
	if !ProtocolMatches(ProtocolAll, ProtocolUDP) {
		t.Fatal("all rule did not match udp traffic")
	}
}

func TestRuleMatchesInsideInterval(t *testing.T) {
	rule := Rule{Number: 100, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 8000, ToPort: 8080}
	if !RuleMatches(rule, ProtocolTCP, 8000) {
		t.Fatal("lower bound was not matched")
	}
	if !RuleMatches(rule, ProtocolTCP, 8080) {
		t.Fatal("upper bound was not matched")
	}
	if !RuleMatches(rule, ProtocolTCP, 8042) {
		t.Fatal("interior port was not matched")
	}
}

func TestRuleMatchesSinglePort(t *testing.T) {
	rule := Rule{Number: 110, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 443, ToPort: 443}
	if !RuleMatches(rule, ProtocolTCP, 443) {
		t.Fatal("the declared port was not matched")
	}
}

func TestRuleMatchesIgnoresPortForICMP(t *testing.T) {
	rule := Rule{Number: 120, Action: ActionAllow, Protocol: ProtocolICMP}
	if !RuleMatches(rule, ProtocolICMP, 0) {
		t.Fatal("icmp rule did not match icmp traffic")
	}
	if RuleMatches(rule, ProtocolTCP, 443) {
		t.Fatal("icmp rule matched tcp traffic")
	}
}

func TestRuleMatchesProtocolGate(t *testing.T) {
	rule := Rule{Number: 130, Action: ActionAllow, Protocol: ProtocolUDP, FromPort: 53, ToPort: 53}
	if RuleMatches(rule, ProtocolTCP, 53) {
		t.Fatal("udp rule matched tcp traffic")
	}
}
