// Package rangehdr turns a client supplied Range header into the byte
// intervals that a response should carry.
package rangehdr

import (
	"strconv"
	"strings"
)

// Span is a byte interval with a start and an end position.
type Span struct {
	Start int64
	End   int64
}

// Len reports how many bytes the span covers.
func (s Span) Len() int64 {
	return s.End - s.Start + 1
}

// Outcome describes how the caller should treat a parsed Range header.
type Outcome int

const (
	// OutcomeNone means no Range header was supplied.
	OutcomeNone Outcome = iota
	// OutcomeRanges means Spans holds the intervals to serve.
	OutcomeRanges
	// OutcomeIgnored means the header must be dropped and the whole object
	// served instead.
	OutcomeIgnored
	// OutcomeUnsatisfiable means the header cannot be met.
	OutcomeUnsatisfiable
	// OutcomeInvalid means the header is malformed and must be rejected.
	OutcomeInvalid
)

// maxRanges is the largest range set the edge accepts; longer sets are
// ignored wholesale.
const maxRanges = 4

// Spec is the parsed form of a Range header.
type Spec struct {
	Outcome Outcome
	Spans   []Span
}

// Parse interprets header against an object of size bytes.
//
// A header whose unit is not bytes yields OutcomeIgnored. Spans are expressed
// in closed form and are returned in the same order as the header lists them.
func Parse(header string, size int64) Spec {
	v := strings.TrimSpace(header)
	if v == "" {
		return Spec{Outcome: OutcomeNone}
	}

	unit, set, ok := strings.Cut(v, "=")
	if !ok || !strings.EqualFold(strings.TrimSpace(unit), "bytes") {
		return Spec{Outcome: OutcomeIgnored}
	}

	parts := strings.Split(set, ",")
	if len(parts) > maxRanges {
		return Spec{Outcome: OutcomeIgnored}
	}

	spans := make([]Span, 0, len(parts))
	for _, part := range parts {
		span, outcome := parsePart(strings.TrimSpace(part), size)
		if outcome != OutcomeRanges {
			return Spec{Outcome: outcome}
		}
		spans = append(spans, span)
	}
	return Spec{Outcome: OutcomeRanges, Spans: spans}
}

// parsePart parses one comma separated range part against an object of size
// bytes and reports the closed interval it selects. A malformed part yields
// OutcomeInvalid, an unsatisfiable one OutcomeUnsatisfiable.
func parsePart(part string, size int64) (Span, Outcome) {
	first, last, ok := strings.Cut(part, "-")
	if !ok {
		return Span{}, OutcomeInvalid
	}
	first = strings.TrimSpace(first)
	last = strings.TrimSpace(last)

	if first == "" {
		n, err := strconv.ParseInt(last, 10, 64)
		if err != nil {
			return Span{}, OutcomeInvalid
		}
		if n <= 0 {
			return Span{}, OutcomeUnsatisfiable
		}
		start := size - n
		if start < 0 {
			start = 0
		}
		return Span{Start: start, End: size - 1}, OutcomeRanges
	}

	start, err := strconv.ParseInt(first, 10, 64)
	if err != nil {
		return Span{}, OutcomeInvalid
	}

	end := size - 1
	if last != "" {
		end, err = strconv.ParseInt(last, 10, 64)
		if err != nil {
			return Span{}, OutcomeInvalid
		}
	}

	if start > end || start >= size {
		return Span{}, OutcomeUnsatisfiable
	}
	if end > size-1 {
		end = size - 1
	}
	return Span{Start: start, End: end}, OutcomeRanges
}
