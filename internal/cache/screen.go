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

	owner := msg.Answers[0].Name.String()
	if !inBailiwick(owner, qname.String()) {
		d.Rejected = append(d.Rejected, owner)
		return d
	}

	for _, rr := range msg.Answers {
		d.Cacheable = append(d.Cacheable, Entry{
			Owner:  rr.Name.String(),
			Type:   rr.Type,
			TTL:    Clamp(int32(rr.TTL)),
			RData:  rr.Data,
			Source: "answer",
		})
	}
	return d
}

// inBailiwick reports whether owner is equal to or below zone.
func inBailiwick(owner, zone string) bool {
	if owner == zone {
		return true
	}
	return strings.HasSuffix(owner, zone)
}
