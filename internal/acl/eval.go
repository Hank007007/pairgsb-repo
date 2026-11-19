package acl

import (
	"errors"
	"fmt"
	"net/netip"
	"sync"
)

// Direction selects which rule book of the target group applies.
type Direction int

const (
	// Ingress matches traffic entering the target group.
	Ingress Direction = iota + 1
	// Egress matches traffic leaving the target group.
	Egress
)

func (d Direction) String() string {
	switch d {
	case Ingress:
		return "ingress"
	case Egress:
		return "egress"
	default:
		return fmt.Sprintf("direction(%d)", int(d))
	}
}

// Flow identifies one direction of a packet: both addresses, both ports and
// the protocol. Flow is the key the stateful connection table is built from.
type Flow struct {
	Src      netip.Addr
	Dst      netip.Addr
	SrcPort  int
	DstPort  int
	Protocol Protocol
}

// Query is one question put to the evaluator: may the flow reach DstPort on
// the target group, judged against that group's ingress or egress book?
type Query struct {
	GroupID   string
	Direction Direction
	Flow      Flow
}

// Decision is the answer to a Query.
type Decision struct {
	Allowed  bool
	Matched  bool
	Rule     RuleNumber
	Stateful bool
}

// ConnectionTable holds the connections the evaluator considers established.
// Flows are recorded in the direction they were initiated.
type ConnectionTable struct {
	mu    sync.RWMutex
	flows []Flow
}

// NewConnectionTable returns an empty table.
func NewConnectionTable() *ConnectionTable {
	return &ConnectionTable{}
}

// Establish records an initiated flow.
func (t *ConnectionTable) Establish(f Flow) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.flows = append(t.flows, f)
	t.mu.Unlock()
}

// Reset drops every recorded connection.
func (t *ConnectionTable) Reset() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.flows = nil
	t.mu.Unlock()
}

// Established reports whether pkt belongs to a flow the table considers
// established.
func (t *ConnectionTable) Established(pkt Flow) bool {
	if t == nil {
		return false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return checkEstablished(t.flows, pkt)
}

func checkEstablished(flows []Flow, pkt Flow) bool {
	for _, f := range flows {
		if f.Src == pkt.Src && f.Dst == pkt.Dst {
			return true
		}
	}
	return false
}

// Evaluator answers connectivity questions against a pinned snapshot of the
// policy inputs, with an optional stateful connection table.
type Evaluator struct {
	source SnapshotSource
	state  *ConnectionTable
}

// NewEvaluator builds an evaluator over the given snapshot source.
func NewEvaluator(source SnapshotSource) *Evaluator {
	return &Evaluator{source: source, state: NewConnectionTable()}
}

// Connections exposes the stateful connection table.
func (e *Evaluator) Connections() *ConnectionTable {
	if e == nil {
		return nil
	}
	return e.state
}

// Evaluate answers q. The whole answer is computed against a single snapshot
// of the policy inputs.
func (e *Evaluator) Evaluate(q Query) (Decision, error) {
	if e == nil || e.source == nil {
		return Decision{}, errors.New("acl: evaluator has no snapshot source")
	}
	proto, err := ParseProtocol(string(q.Flow.Protocol))
	if err != nil {
		return Decision{}, err
	}
	flow := q.Flow
	flow.Protocol = proto

	snap := e.source.Snapshot()
	if _, ok := snap.SecurityGroup(q.GroupID); !ok {
		return Decision{}, fmt.Errorf("acl: unknown security group %q", q.GroupID)
	}

	if e.state != nil && e.state.Established(flow) {
		return Decision{Allowed: true, Stateful: true}, nil
	}

	return evaluateFlow(snap, q.GroupID, q.Direction, flow)
}

func evaluateFlow(snap *Snapshot, groupID string, dir Direction, flow Flow) (Decision, error) {
	sg, ok := snap.SecurityGroup(groupID)
	if !ok {
		return Decision{}, fmt.Errorf("acl: unknown security group %q", groupID)
	}

	var book RuleBook
	switch dir {
	case Ingress:
		book = sg.Ingress
	case Egress:
		book = sg.Egress
	default:
		return Decision{}, fmt.Errorf("acl: unknown direction %v", dir)
	}

	for _, rule := range book {
		if !RuleMatches(rule, flow.Protocol, flow.DstPort) {
			continue
		}
		if !sourceCovers(snap, rule.Source, flow.Src) {
			continue
		}
		return Decision{
			Allowed: rule.Action == ActionAllow,
			Matched: true,
			Rule:    rule.Number,
		}, nil
	}

	return Decision{Allowed: false}, nil
}

// sourceCovers reports whether addr belongs to the set named by src in the
// pinned snapshot.
func sourceCovers(snap *Snapshot, src Source, addr netip.Addr) bool {
	switch src.Kind {
	case SourceCIDR:
		return Contains([]string{src.CIDR}, addr)
	case SourcePrefixList:
		list, ok := snap.PrefixList(src.PrefixList)
		if !ok {
			return false
		}
		return list.Contains(addr)
	case SourceGroup:
		for _, m := range snap.GroupMembers(src.Group) {
			if m.Address.Unmap() == addr.Unmap() {
				return true
			}
		}
		return false
	default:
		return false
	}
}
