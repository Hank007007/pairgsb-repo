// Command maskseal writes and inspects audit journals.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"maskseal-journal/internal/journal"
	"maskseal-journal/internal/keystore"
	"maskseal-journal/internal/policy"
	"maskseal-journal/internal/record"
	"maskseal-journal/internal/unredact"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "append":
		os.Exit(runAppend(os.Args[2:]))
	case "verify":
		os.Exit(runVerify(os.Args[2:]))
	case "unredact":
		os.Exit(runUnredact(os.Args[2:]))
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "maskseal: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: maskseal <command> [flags]

commands:
  append     append a demonstration record
  verify     verify the journal chain
  unredact   retrieve one redacted field after approval
`)
}

func runAppend(args []string) int {
	fs := flag.NewFlagSet("append", flag.ContinueOnError)
	dir := fs.String("journal", "journal-data", "journal directory")
	saltDir := fs.String("salt-dir", "salt-data", "salt store directory")
	policyPath := fs.String("policy", "", "policy file (empty uses the built-in demo rules)")
	actor := fs.String("actor", "alice@example.com", "actor identity")
	action := fs.String("action", "login", "action name")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var rules *policy.RuleSet
	if *policyPath != "" {
		loaded, err := policy.Load(*policyPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "maskseal: %v\n", err)
			return 1
		}
		rules = loaded
	} else {
		rules, _ = policy.Parse([]byte(`{"rules":[{"path":"user.password"},{"path":"items[].ssn"},{"path":"password"}]}`))
	}

	salts, err := keystore.Open(*saltDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "maskseal: %v\n", err)
		return 1
	}
	j, err := journal.Open(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "maskseal: %v\n", err)
		return 1
	}
	defer j.Close()

	fields := map[string]any{
		"user": map[string]any{
			"id":       "u-1042",
			"name":     "Zoë Ångström 🚀",
			"password": "correct horse battery staple",
			"role":     "admin",
		},
		"items": []any{
			map[string]any{"sku": "A-1", "ssn": "111-22-3333"},
			map[string]any{"sku": "B-2", "ssn": "444-55-6666"},
		},
		"source": "demo",
	}
	redactedFields, _, err := record.Redact(fields, rules, salts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "maskseal: %v\n", err)
		return 1
	}
	stored, err := j.Append(record.Record{
		Actor:  *actor,
		Action: *action,
		Time:   time.Now().UTC(),
		Fields: redactedFields,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "maskseal: %v\n", err)
		return 1
	}
	fmt.Printf("appended seq %d hash %s\n", stored.Seq, stored.Hash)
	return 0
}

func runVerify(args []string) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	dir := fs.String("journal", "journal-data", "journal directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	result, err := journal.Verify(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "maskseal: verify failed: %v\n", err)
		return 1
	}
	fmt.Printf("verified %d records, last seq %d\n", result.Records, result.LastSeq)
	return 0
}

func runUnredact(args []string) int {
	fs := flag.NewFlagSet("unredact", flag.ContinueOnError)
	dir := fs.String("journal", "journal-data", "journal directory")
	saltDir := fs.String("salt-dir", "salt-data", "salt store directory")
	seq := fs.Int64("seq", 0, "record sequence number")
	field := fs.String("field", "", "field path")
	requester := fs.String("requester", "", "requester identity")
	approverID := fs.String("approver", "", "approver identity")
	key := fs.String("key", "demo-approval-key", "approval signing key")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *seq <= 0 || strings.TrimSpace(*field) == "" || strings.TrimSpace(*requester) == "" || strings.TrimSpace(*approverID) == "" {
		fmt.Fprintln(os.Stderr, "maskseal: unredact needs -seq, -field, -requester and -approver")
		return 2
	}

	salts, err := keystore.Open(*saltDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "maskseal: %v\n", err)
		return 1
	}
	j, err := journal.Open(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "maskseal: %v\n", err)
		return 1
	}
	defer j.Close()

	req := unredact.Request{
		Seq:       *seq,
		Field:     *field,
		Requester: unredact.Identity{ID: *requester, DisplayName: *requester},
	}
	approver := &unredact.Approver{SigningKey: []byte(*key)}
	tok, err := approver.Approve(req, unredact.Identity{ID: *approverID, DisplayName: *approverID})
	if err != nil {
		fmt.Fprintf(os.Stderr, "maskseal: approval rejected: %v\n", err)
		return 1
	}
	registry := unredact.NewRegistry()
	result, err := unredact.Unredact(j, salts, approver, registry, req, tok, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "maskseal: unredact failed: %v\n", err)
		return 1
	}
	fmt.Printf("seq %d field %s value %q (audit seq %d)\n", result.Seq, result.Field, result.Plaintext, result.AuditSeq)
	return 0
}
