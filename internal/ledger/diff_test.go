package ledger

import (
	"path/filepath"
	"testing"
	"time"
)

func highFinding(rule, file string, line int) Finding {
	return Finding{RuleID: rule, File: file, Line: line, Severity: SeverityHigh}
}

func TestDiffReportsFindingsAbsentFromBaseline(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(filepath.Join(dir, "ledger.jsonl"), testKey())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()

	baseline := []Finding{highFinding("go/sql-injection", "internal/db/query.go", 42)}
	current := []Finding{
		highFinding("go/sql-injection", "internal/db/query.go", 42),
		highFinding("go/hardcoded-secret", "internal/config/config.go", 11),
	}

	result, err := l.Diff(current, baseline, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.New) != 1 {
		t.Fatalf("got %d new findings, want 1", len(result.New))
	}
	if result.New[0].RuleID != "go/hardcoded-secret" {
		t.Fatalf("unexpected new finding: %+v", result.New[0])
	}
	if result.NewHigh != 1 {
		t.Fatalf("NewHigh = %d, want 1", result.NewHigh)
	}
	if len(result.Resolved) != 0 {
		t.Fatalf("got %d resolved findings, want 0", len(result.Resolved))
	}
}

func TestDiffIgnoresLowSeverityForExitCode(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(filepath.Join(dir, "ledger.jsonl"), testKey())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	current := []Finding{{RuleID: "go/style", File: "a.go", Line: 3, Severity: SeverityLow}}

	result, err := l.Diff(current, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.New) != 1 {
		t.Fatalf("got %d new findings, want 1", len(result.New))
	}
	if result.NewHigh != 0 {
		t.Fatalf("NewHigh = %d, want 0", result.NewHigh)
	}
}

func TestDiffDeduplicatesRepeatedFindings(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(filepath.Join(dir, "ledger.jsonl"), testKey())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	f := highFinding("go/sql-injection", "internal/db/query.go", 42)

	result, err := l.Diff([]Finding{f, f}, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.New) != 1 {
		t.Fatalf("got %d new findings, want 1", len(result.New))
	}
	if result.NewHigh != 1 {
		t.Fatalf("NewHigh = %d, want 1", result.NewHigh)
	}
}

func TestDiffEmptyInputs(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(filepath.Join(dir, "ledger.jsonl"), testKey())
	if err != nil {
		t.Fatal(err)
	}
	result, err := l.Diff(nil, nil, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.New) != 0 || len(result.Resolved) != 0 || result.NewHigh != 0 {
		t.Fatalf("empty inputs produced a non-empty result: %+v", result)
	}
}
