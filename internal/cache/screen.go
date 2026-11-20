package cache

import (
	"strings"

	"ptrloop/internal/wire"
)

// Entry is one record that passed screening and may be stored.
type Entry struct {
	Owner  string
	Type   uint16
	TTL    TTL
	RData  wire.RData
	Source string
}

// Decision is the outcome of screening one response.
type Decision struct {
	Owner     string
	Cacheable []Entry
	Rejected  []string
}

// Screen decides which answer records from msg may be stored for the query
// name qname. Records that fail screening are listed in Rejected.
func Screen(msg *wire.Message, qname wire.Name) Decision {
	d := Decision{Owner: qname.String()}
	if msg == nil || len(msg.Answers) == 0 {
		return d
	}

	qnameNorm, zoneOK := normalizeName(qname)
	if !zoneOK {
		for _, rr := range msg.Answers {
			d.Rejected = append(d.Rejected, rr.Name.String())
		}
		return d
	}
	zone := admissionZone(qnameNorm)

	type answer struct {
		rr    wire.Record
		owner string
		ok    bool
	}
	answers := make([]answer, len(msg.Answers))
	for i, rr := range msg.Answers {
		owner, ok := normalizeName(rr.Name)
		answers[i] = answer{rr: rr, owner: owner, ok: ok}
	}

	// Walk the CNAME chain from qname. A record is cacheable only when it is
	// reachable from the question name and every hop stays in bailiwick.
	used := make([]bool, len(answers))
	seen := make(map[string]bool)
	current := qnameNorm
	firstHop := true
	rejectChain := false

	for {
		if seen[current] {
			rejectChain = true
			break
		}
		seen[current] = true

		var atCurrent []int
		for i, a := range answers {
			if !used[i] && a.ok && a.owner == current {
				atCurrent = append(atCurrent, i)
			}
		}
		if len(atCurrent) == 0 {
			break
		}

		if !inBailiwick(current, zone) {
			rejectChain = true
			break
		}
		if firstHop {
			for _, i := range atCurrent {
				if !echoesCase(qname, answers[i].rr.Name) {
					rejectChain = true
					break
				}
			}
			if rejectChain {
				break
			}
		}

		var targets []string
		for _, i := range atCurrent {
			used[i] = true
			rr := answers[i].rr
			if rr.Type != wire.TypeCNAME {
				continue
			}
			if len(rr.Data.Names) != 1 {
				rejectChain = true
				break
			}
			target, ok := normalizeName(rr.Data.Names[0])
			if !ok {
				rejectChain = true
				break
			}
			targets = append(targets, target)
		}
		if rejectChain {
			break
		}

		if len(targets) == 0 {
			break
		}
		for _, target := range targets[1:] {
			if target != targets[0] {
				rejectChain = true
				break
			}
		}
		if rejectChain {
			break
		}
		current = targets[0]
		firstHop = false
	}

	if rejectChain {
		for _, a := range answers {
			d.Rejected = append(d.Rejected, a.rr.Name.String())
		}
		return d
	}

	for i, a := range answers {
		if !used[i] {
			continue
		}
		d.Cacheable = append(d.Cacheable, Entry{
			Owner:  a.rr.Name.String(),
			Type:   a.rr.Type,
			TTL:    Clamp(a.rr.TTL),
			RData:  a.rr.Data,
			Source: "answer",
		})
	}
	for i, a := range answers {
		if !used[i] {
			d.Rejected = append(d.Rejected, a.rr.Name.String())
		}
	}
	return d
}

// admissionZone is the part of the question name that bounds cache admission.
// The matches in the specification use the parent domain (example.com for
// www.example.com), never a bare top-level label.
func admissionZone(name string) string {
	labels := strings.Split(name, ".")
	if len(labels) > 2 {
		labels = labels[1:]
	}
	return strings.Join(labels, ".")
}

// inBailiwick reports whether owner is equal to or below zone.
func inBailiwick(owner, zone string) bool {
	owner = strings.TrimSuffix(owner, ".")
	zone = strings.TrimSuffix(zone, ".")
	if owner == zone {
		return true
	}
	if zone == "" {
		return true
	}
	return strings.HasSuffix(owner, "."+zone)
}

// normalizeName lower-cases a name, removes its trailing dot and expands the
// decimal escape form \DDD. It returns false when a label cannot be normalized.
func normalizeName(n wire.Name) (string, bool) {
	labels := make([]string, len(n.Labels))
	for i, label := range n.Labels {
		norm, ok := normalizeLabel(label)
		if !ok {
			return "", false
		}
		labels[i] = norm
	}
	return strings.Join(labels, "."), true
}

func normalizeLabel(label string) (string, bool) {
	if label == "" {
		return "", false
	}
	out := make([]byte, 0, len(label))
	for i := 0; i < len(label); i++ {
		c := label[i]
		if c == '\\' {
			if i+3 >= len(label) {
				return "", false
			}
			hundreds, ok1 := decimalDigit(label[i+1])
			tens, ok2 := decimalDigit(label[i+2])
			ones, ok3 := decimalDigit(label[i+3])
			if !ok1 || !ok2 || !ok3 {
				return "", false
			}
			value := hundreds*100 + tens*10 + ones
			if value > 255 || !isLabelByte(byte(value)) {
				return "", false
			}
			out = append(out, lowerASCII(byte(value)))
			i += 3
			continue
		}
		if !isLabelByte(c) {
			return "", false
		}
		out = append(out, lowerASCII(c))
	}
	if len(out) == 0 || len(out) > 63 || out[0] == '-' {
		return "", false
	}
	return string(out), true
}

func decimalDigit(c byte) (int, bool) {
	if c < '0' || c > '9' {
		return 0, false
	}
	return int(c - '0'), true
}

func isLabelByte(c byte) bool {
	return c >= 'A' && c <= 'Z' ||
		c >= 'a' && c <= 'z' ||
		c >= '0' && c <= '9' ||
		c == '-'
}

func lowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// echoesCase checks the 0x20 echo on the original wire representation. Only
// ASCII letters participate; digits and hyphens are compared indirectly by the
// normalized name match that made the record reachable.
func echoesCase(qname, owner wire.Name) bool {
	if len(qname.Labels) != len(owner.Labels) {
		return false
	}
	for i := range qname.Labels {
		q := qname.Labels[i]
		o := owner.Labels[i]
		if len(q) != len(o) {
			return false
		}
		for j := range q {
			qc, oc := q[j], o[j]
			if isASCIILetter(qc) || isASCIILetter(oc) {
				if qc != oc {
					return false
				}
			}
		}
	}
	return true
}

func isASCIILetter(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}
