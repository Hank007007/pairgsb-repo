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
)

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
	if !ok {
		return Spec{Outcome: OutcomeIgnored}
	}
	if !strings.EqualFold(strings.TrimSpace(unit), "bytes") {
		return Spec{Outcome: OutcomeIgnored}
	}

	parts := strings.Split(set, ",")
	spans := make([]Span, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		first, last, ok := strings.Cut(part, "-")
		if !ok {
			return Spec{Outcome: OutcomeNone}
		}
		first = strings.TrimSpace(first)
		last = strings.TrimSpace(last)

		if first == "" {
			n, err := strconv.ParseInt(last, 10, 64)
			if err != nil {
				return Spec{Outcome: OutcomeNone}
			}
			if n <= 0 {
				spans = append(spans, Span{Start: 0, End: size - 1})
				continue
			}
			spans = append(spans, Span{Start: size - n + 1, End: size})
			continue
		}

		start, err := strconv.ParseInt(first, 10, 64)
		if err != nil {
			return Spec{Outcome: OutcomeNone}
		}

		end := size - 1
		if last != "" {
			end, err = strconv.ParseInt(last, 10, 64)
			if err != nil {
				return Spec{Outcome: OutcomeNone}
			}
			if end > size-1 {
				end = size - 1
			}
		}

		if start > end {
			continue
		}
		if start >= size {
			continue
		}
		spans = append(spans, Span{Start: start, End: end})
	}

	if len(spans) == 0 {
		return Spec{Outcome: OutcomeNone}
	}
	return Spec{Outcome: OutcomeRanges, Spans: spans}
}
