package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunCountsOneRecordPerLine(t *testing.T) {
	in := strings.Join([]string{
		`timestamp=2026-03-14T09:00:00Z service=billing-api level=info`,
		`timestamp=2026-03-14T09:00:01Z service=billing-api level=warn`,
		"",
	}, "\n")

	var out bytes.Buffer
	n, err := run(strings.NewReader(in), &out, false)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if n != 2 {
		t.Fatalf("run counted %d events, want 2", n)
	}
}

func TestRunWithKeysListsNames(t *testing.T) {
	in := "timestamp=2026-03-14T09:00:00Z service=billing-api\n"

	var out bytes.Buffer
	if _, err := run(strings.NewReader(in), &out, true); err != nil {
		t.Fatalf("run: %v", err)
	}

	want := "event 1: timestamp,service\n"
	if out.String() != want {
		t.Fatalf("run output = %q, want %q", out.String(), want)
	}
}
