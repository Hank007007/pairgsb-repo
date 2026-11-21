package ledger

import (
	"fmt"
	"sort"
	"time"
)

// Baseline is a previously accepted snapshot of a scan, used by CI to detect
// findings that were not there before.
type Baseline struct {
	Findings []Finding `json:"findings"`
}

// DiffEntry describes one finding touched by a diff run.
type DiffEntry struct {
	Fingerprint string   `json:"fingerprint"`
	RuleID      string   `json:"ruleId"`
	File        string   `json:"file"`
	Line        int      `json:"line"`
	Severity    Severity `json:"severity"`
	State       string   `json:"state"`
}

// DiffResult is the outcome of comparing a scan against the ledger and an
// optional baseline.
type DiffResult struct {
	New      []DiffEntry `json:"new"`
	Resolved []DiffEntry `json:"resolved"`
	NewHigh  int         `json:"newHigh"`
}

// Diff compares the findings of the current scan with the ledger state and the
// baseline. A finding covered by an active waiver is not reported; a finding
// already present in the baseline is not reported as new.
func (l *Ledger) Diff(current []Finding, baseline []Finding, now time.Time) (DiffResult, error) {
	var result DiffResult

	seen := make(map[string]Finding, len(current))
	for _, f := range current {
		fp := f.Fingerprint()
		if _, ok := seen[fp]; ok {
			continue
		}
		seen[fp] = f
	}

	baselineSet := make(map[string]Finding, len(baseline))
	for _, f := range baseline {
		fp := f.Fingerprint()
		if _, ok := baselineSet[fp]; ok {
			continue
		}
		baselineSet[fp] = f
	}

	for fp, f := range seen {
		state := l.State(fp)
		eff := state.EffectiveStatus(now)
		if eff == StatusWaived {
			continue
		}
		if _, inBaseline := baselineSet[fp]; inBaseline {
			continue
		}
		result.New = append(result.New, DiffEntry{
			Fingerprint: fp,
			RuleID:      f.RuleID,
			File:        f.File,
			Line:        f.Line,
			Severity:    f.Severity,
			State:       string(eff),
		})
	}

	for fp, f := range baselineSet {
		if _, still := seen[fp]; still {
			continue
		}
		result.Resolved = append(result.Resolved, DiffEntry{
			Fingerprint: fp,
			RuleID:      f.RuleID,
			File:        f.File,
			Line:        f.Line,
			Severity:    f.Severity,
			State:       string(StatusFixed),
		})
	}

	sortEntries(result.New)
	sortEntries(result.Resolved)

	for _, e := range result.New {
		if e.Severity.IsHigh() && countsTowardsNewHigh(Status(e.State)) {
			result.NewHigh++
		}
	}

	return result, nil
}

func sortEntries(entries []DiffEntry) {
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.Fingerprint != b.Fingerprint {
			return a.Fingerprint < b.Fingerprint
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
}

func countsTowardsNewHigh(s Status) bool {
	switch s {
	case StatusFixed, StatusWaived, Status("resolved"):
		return false
	}
	return true
}

// RecordDiff writes the ledger records implied by a diff result.
func (l *Ledger) RecordDiff(result DiffResult, now time.Time) error {
	for _, e := range result.Resolved {
		state := l.State(e.Fingerprint)
		if !state.Known {
			continue
		}
		if state.Status == StatusFixed {
			continue
		}
		r := l.NewRecord(KindResolved)
		r.Fingerprint = e.Fingerprint
		r.FindingID = FindingID(e.Fingerprint)
		r.RuleID = e.RuleID
		r.File = e.File
		r.Line = e.Line
		r.Status = state.Status
		if err := TransitionAt(&r, StatusFixed, now); err != nil {
			return fmt.Errorf("resolve %s from %q: %w", e.Fingerprint, state.Status, err)
		}
		if err := l.Append(r); err != nil {
			return fmt.Errorf("append resolved record for %s: %w", e.Fingerprint, err)
		}
	}
	return nil
}
