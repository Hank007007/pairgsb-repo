package acl

import (
	"errors"
	"strings"

	"topicwarden/internal/tenant"
)

// ErrMalformedSharedFilter indicates an invalid $share subscription.
var ErrMalformedSharedFilter = errors.New("malformed shared subscription filter")

// SharedPrefix is the first level marking a shared subscription.
const SharedPrefix = "$share"

// SharedSubscription is a parsed $share/<group>/<filter> subscription.
type SharedSubscription struct {
	Group  string
	Filter string
}

// ParseSharedFilter unwraps a shared subscription filter.
//
// The boolean reports whether filter uses the shared-subscription form. A
// shared filter with an empty group or empty inner filter is reported as
// malformed. A nested "$share/" prefix after the group is part of the literal
// inner filter.
func ParseSharedFilter(filter string) (shared SharedSubscription, isShared bool, err error) {
	normalized := tenant.NormalizeTopic(filter)
	levels := strings.Split(normalized, "/")
	if len(levels) == 0 || levels[0] != SharedPrefix {
		return SharedSubscription{Filter: normalized}, false, nil
	}

	if len(levels) < 3 || levels[1] == "" {
		return SharedSubscription{}, true, ErrMalformedSharedFilter
	}

	inner := strings.Join(levels[2:], "/")
	if inner == "" {
		return SharedSubscription{}, true, ErrMalformedSharedFilter
	}

	return SharedSubscription{Group: levels[1], Filter: inner}, true, nil
}

// UnwrapSharedFilter returns the inner filter for a valid shared subscription
// and reports whether unwrapping occurred. Malformed shared filters return
// ok=false.
func UnwrapSharedFilter(filter string) (inner string, ok bool) {
	parsed, _, err := ParseSharedFilter(filter)
	if err != nil {
		return "", false
	}
	return parsed.Filter, true
}
