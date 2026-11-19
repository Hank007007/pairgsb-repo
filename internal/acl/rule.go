// Package acl implements the security-group evaluator used by the private
// cloud control plane. The orchestrator calls it before publishing rules and
// the drift detector calls it while auditing live traffic; both ask the same
// question: may a given source reach a given protocol and port on a target
// security group.
//
// The normative description of the evaluation semantics lives in
// docs/eval-semantics.md.
package acl

import "fmt"

// Protocol is a transport protocol name understood by the evaluator.
type Protocol string

const (
	ProtocolTCP  Protocol = "tcp"
	ProtocolUDP  Protocol = "udp"
	ProtocolICMP Protocol = "icmp"
	ProtocolAll  Protocol = "all"
)

// ParseProtocol validates a protocol name.
func ParseProtocol(s string) (Protocol, error) {
	switch Protocol(s) {
	case ProtocolTCP, ProtocolUDP, ProtocolICMP, ProtocolAll:
		return Protocol(s), nil
	default:
		return "", fmt.Errorf("acl: unknown protocol %q", s)
	}
}

// RuleNumber is the position of a rule inside a rule book.
//
// Numbers are unique within one direction of one security group and they
// order the evaluation of the book. They are plain numbers: ordering is
// numeric, not lexicographic.
type RuleNumber int

// Action is the outcome of a matched rule.
type Action int

const (
	// ActionAllow lets the traffic through.
	ActionAllow Action = iota + 1
	// ActionDeny drops the traffic.
	ActionDeny
)

func (a Action) String() string {
	switch a {
	case ActionAllow:
		return "allow"
	case ActionDeny:
		return "deny"
	default:
		return fmt.Sprintf("action(%d)", int(a))
	}
}

// SourceKind tells which union member of Source is populated.
type SourceKind int

const (
	SourceCIDR SourceKind = iota + 1
	SourcePrefixList
	SourceGroup
)

// Source describes where traffic may originate. Exactly one of the fields
// that belongs to Kind is meaningful.
type Source struct {
	Kind       SourceKind
	CIDR       string
	PrefixList string
	Group      string
}

// CIDRSource builds a source backed by a literal CIDR block.
func CIDRSource(cidr string) Source {
	return Source{Kind: SourceCIDR, CIDR: cidr}
}

// PrefixListSource builds a source backed by a named prefix list.
func PrefixListSource(name string) Source {
	return Source{Kind: SourcePrefixList, PrefixList: name}
}

// GroupSource builds a source backed by another security group.
func GroupSource(groupID string) Source {
	return Source{Kind: SourceGroup, Group: groupID}
}

// Rule is a single ingress or egress rule.
//
// Ports form an inclusive interval. They are only interpreted for tcp and
// udp; for icmp and all they carry no meaning.
type Rule struct {
	Number   RuleNumber
	Action   Action
	Protocol Protocol
	FromPort int
	ToPort   int
	Source   Source
}

// RuleBook holds the rules of one direction, keyed by rule number.
type RuleBook map[RuleNumber]Rule

// NewRuleBook builds a rule book from rules, keyed by their numbers.
func NewRuleBook(rules ...Rule) RuleBook {
	book := make(RuleBook, len(rules))
	for _, rule := range rules {
		book[rule.Number] = rule
	}
	return book
}

// SecurityGroup is a named set of ingress and egress rules.
type SecurityGroup struct {
	ID      string
	Ingress RuleBook
	Egress  RuleBook
}
