// Command scanctl drives the findings ledger from CI and from the terminal.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"findingsledger/internal/ledger"
)

const (
	exitOK       = 0
	exitFindings = 1
	exitError    = 2
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: scanctl <command> [flags]

commands:
  ingest   feed a scanner export into the ledger
  diff     compare a scan against the ledger and an optional baseline
  waiver   record a waiver for a finding
  verify   recompute the ledger and check its checkpoints

`)
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return exitError
	}

	cmd, rest := args[0], args[1:]

	key, err := ledger.LoadKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "scanctl: %v\n", err)
		return exitError
	}

	switch cmd {
	case "ingest":
		return cmdIngest(rest, key)
	case "diff":
		return cmdDiff(rest, key)
	case "waiver":
		return cmdWaiver(rest, key)
	case "verify":
		return cmdVerify(rest, key)
	case "-h", "--help", "help":
		usage()
		return exitOK
	default:
		fmt.Fprintf(os.Stderr, "scanctl: unknown command %q\n", cmd)
		usage()
		return exitError
	}
}

func openLedger(path string, key []byte) (*ledger.Ledger, error) {
	if path == "" {
		return nil, errors.New("--ledger is required")
	}
	return ledger.Open(path, key)
}

func cmdIngest(args []string, key []byte) int {
	fs := flag.NewFlagSet("ingest", flag.ContinueOnError)
	ledgerPath := fs.String("ledger", "", "path to the ledger file")
	scanPath := fs.String("scan", "", "path to a scanner export")
	if err := fs.Parse(args); err != nil {
		return exitError
	}
	if *scanPath == "" {
		fmt.Fprintln(os.Stderr, "scanctl ingest: --scan is required")
		return exitError
	}

	l, err := openLedger(*ledgerPath, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scanctl ingest: %v\n", err)
		return exitError
	}
	findings, err := ledger.LoadScan(*scanPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scanctl ingest: %v\n", err)
		return exitError
	}

	now := time.Now().UTC()
	var added, reopened int
	for _, f := range findings {
		fp := f.Fingerprint()
		state := l.State(fp)
		var kind ledger.RecordKind
		var target ledger.Status
		switch {
		case !state.Known:
			kind = ledger.KindObservation
			target = ledger.StatusOpen
			added++
		case state.Status == ledger.StatusFixed:
			kind = ledger.KindReopened
			target = ledger.StatusReopened
			reopened++
		default:
			continue
		}
		r := l.NewRecord(kind)
		r.Fingerprint = fp
		r.FindingID = ledger.FindingID(fp)
		r.RuleID = f.RuleID
		r.File = f.File
		r.Line = f.Line
		r.Status = state.Status
		if err := ledger.TransitionAt(&r, target, now); err != nil {
			fmt.Fprintf(os.Stderr, "scanctl ingest: %v\n", err)
			return exitError
		}
		if err := l.Append(r); err != nil {
			fmt.Fprintf(os.Stderr, "scanctl ingest: %v\n", err)
			return exitError
		}
	}

	fmt.Printf("ingested %d findings: %d new, %d reopened\n", len(findings), added, reopened)
	return exitOK
}

func cmdDiff(args []string, key []byte) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	ledgerPath := fs.String("ledger", "", "path to the ledger file")
	scanPath := fs.String("scan", "", "path to a scanner export")
	baselinePath := fs.String("baseline", "", "path to a baseline scan")
	asJSON := fs.Bool("json", false, "emit the diff result as JSON")
	if err := fs.Parse(args); err != nil {
		return exitError
	}
	if *scanPath == "" {
		fmt.Fprintln(os.Stderr, "scanctl diff: --scan is required")
		return exitError
	}

	l, err := openLedger(*ledgerPath, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scanctl diff: %v\n", err)
		return exitError
	}
	current, err := ledger.LoadScan(*scanPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scanctl diff: %v\n", err)
		return exitError
	}
	var baseline []ledger.Finding
	if *baselinePath != "" {
		baseline, err = ledger.LoadScan(*baselinePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scanctl diff: %v\n", err)
			return exitError
		}
	}

	now := time.Now().UTC()
	result, err := l.Diff(current, baseline, now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scanctl diff: %v\n", err)
		return exitError
	}
	if err := l.RecordDiff(result, now); err != nil {
		fmt.Fprintf(os.Stderr, "scanctl diff: %v\n", err)
		return exitError
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "scanctl diff: %v\n", err)
			return exitError
		}
	} else {
		for _, e := range result.New {
			fmt.Printf("new\t%s\t%s\t%s:%d\n", e.Severity, e.Fingerprint, e.File, e.Line)
		}
		for _, e := range result.Resolved {
			fmt.Printf("resolved\t%s\t%s:%d\n", e.Fingerprint, e.File, e.Line)
		}
		fmt.Printf("%d new findings, %d high-risk\n", len(result.New), result.NewHigh)
	}

	if result.NewHigh > 0 {
		return exitFindings
	}
	return exitOK
}

func cmdWaiver(args []string, key []byte) int {
	fs := flag.NewFlagSet("waiver", flag.ContinueOnError)
	ledgerPath := fs.String("ledger", "", "path to the ledger file")
	fingerprint := fs.String("fingerprint", "", "fingerprint of the finding")
	reason := fs.String("reason", "", "why the finding is accepted")
	submittedBy := fs.String("submitted-by", "", "person requesting the waiver")
	approver := fs.String("approver", "", "person approving the waiver")
	expiresIn := fs.Duration("expires-in", ledger.DefaultWaiverTTL, "waiver lifetime")
	if err := fs.Parse(args); err != nil {
		return exitError
	}
	if *fingerprint == "" || *reason == "" || *submittedBy == "" {
		fmt.Fprintln(os.Stderr, "scanctl waiver: --fingerprint, --reason and --submitted-by are required")
		return exitError
	}

	l, err := openLedger(*ledgerPath, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scanctl waiver: %v\n", err)
		return exitError
	}
	now := time.Now().UTC()
	w, err := l.SubmitWaiver(*fingerprint, *reason, *submittedBy, *approver, now.Add(*expiresIn), now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scanctl waiver: %v\n", err)
		return exitError
	}
	fmt.Printf("waived %s until %s\n", w.Fingerprint, w.ExpiresAt.Format(time.RFC3339))
	return exitOK
}

func cmdVerify(args []string, key []byte) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	ledgerPath := fs.String("ledger", "", "path to the ledger file")
	if err := fs.Parse(args); err != nil {
		return exitError
	}
	if *ledgerPath == "" {
		fmt.Fprintln(os.Stderr, "scanctl verify: --ledger is required")
		return exitError
	}

	report, err := ledger.Verify(*ledgerPath, key)
	if err != nil {
		if report.BadSeq > 0 {
			fmt.Fprintf(os.Stderr, "scanctl verify: ledger is broken at seq %d: %v\n", report.BadSeq, err)
		} else if report.BadCheckpointSeq > 0 {
			fmt.Fprintf(os.Stderr, "scanctl verify: checkpoint at seq %d is invalid: %v\n", report.BadCheckpointSeq, err)
		} else {
			fmt.Fprintf(os.Stderr, "scanctl verify: %v\n", err)
		}
		return exitFindings
	}

	for _, w := range report.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	fmt.Println(report.Message)
	return exitOK
}
