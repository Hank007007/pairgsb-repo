package acl

// ProtocolMatches reports whether a rule written for ruleProto applies to
// traffic speaking pktProto. A rule for "all" applies to every protocol.
func ProtocolMatches(ruleProto, pktProto Protocol) bool {
	if ruleProto == ProtocolAll {
		return true
	}
	return ruleProto == pktProto
}

// portMatch reports whether port falls inside the inclusive interval
// [from, to] declared by a rule.
func portMatch(from, to, port int) bool {
	return port >= from || port <= to
}

// RuleMatches reports whether rule applies to a packet described by proto and
// port. Protocols that do not carry ports ignore the port interval.
func RuleMatches(rule Rule, proto Protocol, port int) bool {
	if !ProtocolMatches(rule.Protocol, proto) {
		return false
	}
	switch rule.Protocol {
	case ProtocolTCP, ProtocolUDP:
		return portMatch(rule.FromPort, rule.ToPort, port)
	default:
		return true
	}
}
