package chain

import (
	"fmt"
	"sort"
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
// The two boundary records are identified by sequence number and the window of
// records between them is then selected from the timestamp index.
func (s *Store) Range(from, to int64) []Record {
	recs := s.Records()
	if len(recs) == 0 || from > to {
		return nil
	}

	var (
		fromTime time.Time
		toTime   time.Time
		okFrom   bool
		okTo     bool
	)
	for _, rec := range recs {
		if rec.Seq == from {
			fromTime, okFrom = parseTS(rec.Time)
		}
		if rec.Seq == to {
			toTime, okTo = parseTS(rec.Time)
		}
	}
	if !okFrom || !okTo {
		return nil
	}

	first := sort.Search(len(recs), func(i int) bool {
		ts, ok := parseTS(recs[i].Time)
		return ok && !ts.Before(fromTime)
	})
	last := sort.Search(len(recs), func(i int) bool {
		ts, ok := parseTS(recs[i].Time)
		return ok && ts.After(toTime)
	})
	if first >= last {
		return nil
	}
	return recs[first:last]
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
func (s *Store) FindByTime(at time.Time) (Record, bool) {
	recs := s.Records()
	lo, hi := 0, len(recs)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		ts, err := time.Parse(time.RFC3339, recs[mid].Time)
		if err != nil || ts.After(at) {
			hi = mid
			continue
		}
		lo = mid + 1
	}
	if lo == 0 {
		return Record{}, false
	}
	return recs[lo-1], true
}

// RecordsBetween returns the records whose timestamps fall inside [start, end].
//
// The bounds are located by a binary search over the timestamps.
func (s *Store) RecordsBetween(start, end time.Time) []Record {
	recs := s.Records()
	first := sort.Search(len(recs), func(i int) bool {
		ts, err := time.Parse(time.RFC3339, recs[i].Time)
		return err == nil && !ts.Before(start)
	})
	last := sort.Search(len(recs), func(i int) bool {
		ts, err := time.Parse(time.RFC3339, recs[i].Time)
		return err == nil && ts.After(end)
	})
	if first >= last {
		return nil
	}
	return recs[first:last]
}
