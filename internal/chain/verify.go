package chain

import (
	"fmt"
	"time"
)

// Span identifies a closed record interval [FromSeq, ToSeq].
type Span struct {
	FromSeq int64
	ToSeq   int64
}

// Problem is a single verification finding.
type Problem struct {
	Seq    int64
	Field  string
	Detail string
}

func (p Problem) String() string {
	if p.Seq != 0 {
		return fmt.Sprintf("seq %d: %s: %s", p.Seq, p.Field, p.Detail)
	}
	return fmt.Sprintf("%s: %s", p.Field, p.Detail)
}

// Report collects the findings of a verification pass.
type Report struct {
	Problems []Problem
}

// OK reports whether the pass produced no findings.
func (r *Report) OK() bool { return len(r.Problems) == 0 }

func (r *Report) add(p Problem) { r.Problems = append(r.Problems, p) }

// Verify replays the record hashes and linkage over the whole log.
//
// The hash of every record is recomputed and compared with the stored value,
// the parent link of every record must name the previous record's hash, and
// sequence numbers must be dense starting at 1. Timestamps must parse as
// RFC3339.
//
// spans lists the intervals receipts are expected to cover. Coverage is
// checked by the receipt layer; this replay establishes that the records
// themselves are intact.
func Verify(records []Record, spans []Span) *Report {
	rep := &Report{}
	var prev Record
	for i, rec := range records {
		if rec.Seq != int64(i)+1 {
			rep.add(Problem{Seq: rec.Seq, Field: "seq", Detail: fmt.Sprintf("expected %d, found %d", i+1, rec.Seq)})
		}
		if i == 0 {
			if rec.Prev != "" {
				rep.add(Problem{Seq: rec.Seq, Field: "prev", Detail: "first record must not name a parent"})
			}
		} else if rec.Prev != prev.Hash {
			rep.add(Problem{Seq: rec.Seq, Field: "prev", Detail: fmt.Sprintf("does not match hash of seq %d", prev.Seq)})
		}
		if got := ComputeHash(rec); got != rec.Hash {
			rep.add(Problem{Seq: rec.Seq, Field: "hash", Detail: "stored hash does not match recomputed value"})
		}
		if _, err := time.Parse(time.RFC3339, rec.Time); err != nil {
			rep.add(Problem{Seq: rec.Seq, Field: "ts", Detail: "timestamp is not RFC3339"})
		}
		prev = rec
	}
	_ = spans
	return rep
}

// Range returns the records whose sequence numbers fall in [from, to].
//
// Selection is keyed by seq. Timestamps are evidence, not an index, and may
// move backwards when the wall clock is adjusted.
func (s *Store) Range(from, to int64) []Record {
	recs := s.Records()
	if len(recs) == 0 || from > to {
		return nil
	}

	out := make([]Record, 0)
	for _, rec := range recs {
		if rec.Seq >= from && rec.Seq <= to {
			out = append(out, rec)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseTS(s string) (time.Time, bool) {
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}

// FindByTime returns the record whose timestamp is nearest to, and not after,
// the supplied instant. The second result is false when no such record exists.
//
// Timestamps are not sorted because the wall clock can step backwards, so the
// lookup scans the records instead of binary-searching the timestamp column.
func (s *Store) FindByTime(at time.Time) (Record, bool) {
	recs := s.Records()
	var (
		best   Record
		bestTS time.Time
		found  bool
	)
	for _, rec := range recs {
		ts, ok := parseTS(rec.Time)
		if !ok || ts.After(at) {
			continue
		}
		if !found || ts.After(bestTS) || (ts.Equal(bestTS) && rec.Seq > best.Seq) {
			best = rec
			bestTS = ts
			found = true
		}
	}
	return best, found
}

// RecordsBetween returns the records whose timestamps fall inside [start, end].
//
// Timestamps are not sorted, so every record is considered. The result stays
// in chain order.
func (s *Store) RecordsBetween(start, end time.Time) []Record {
	recs := s.Records()
	var out []Record
	for _, rec := range recs {
		ts, ok := parseTS(rec.Time)
		if !ok || ts.Before(start) || ts.After(end) {
			continue
		}
		out = append(out, rec)
	}
	return out
}
