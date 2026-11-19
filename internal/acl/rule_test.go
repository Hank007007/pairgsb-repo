package acl

import (
	"testing"
)

func TestRuleBookKeyedByNumber(t *testing.T) {
	book := NewRuleBook(
		Rule{Number: 100, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 443, ToPort: 443, Source: CIDRSource("10.0.0.0/8")},
		Rule{Number: 200, Action: ActionDeny, Protocol: ProtocolTCP, FromPort: 22, ToPort: 22, Source: CIDRSource("10.0.0.0/8")},
	)
	if len(book) != 2 {
		t.Fatalf("rule book holds %d rules, want 2", len(book))
	}
	if got, ok := book[100]; !ok || got.Action != ActionAllow {
		t.Fatalf("rule 100 = %+v, present=%v", got, ok)
	}
	if got, ok := book[200]; !ok || got.Action != ActionDeny {
		t.Fatalf("rule 200 = %+v, present=%v", got, ok)
	}
}

func TestParseProtocol(t *testing.T) {
	for _, name := range []string{"tcp", "udp", "icmp", "all"} {
		got, err := ParseProtocol(name)
		if err != nil {
			t.Fatalf("ParseProtocol(%q): %v", name, err)
		}
		if string(got) != name {
			t.Fatalf("ParseProtocol(%q) = %q", name, got)
		}
	}
	if _, err := ParseProtocol("sctp"); err == nil {
		t.Fatal("ParseProtocol accepted an unknown protocol")
	}
}

func TestActionString(t *testing.T) {
	if ActionAllow.String() != "allow" || ActionDeny.String() != "deny" {
		t.Fatalf("unexpected action strings: %q %q", ActionAllow, ActionDeny)
	}
}
