package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSplitKeysSkipsQuotedValuesAndEscapes(t *testing.T) {
	line := `timestamp=2026-03-14T09:00:00Z user_agent="Mozilla/5.0 (X11; Linux x86_64) \"Safari\"" message="a b" region=eu-west-1`

	got := splitKeys(line)
	want := []string{"timestamp", "user_agent", "message", "region"}
	if len(got) != len(want) {
		t.Fatalf("splitKeys returned %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitKeys returned %v, want %v", got, want)
		}
	}
}

func TestRunWithKeysKeepsQuotedValuesIntact(t *testing.T) {
	in := "user_agent=\"Mozilla/5.0 (X11; Linux x86_64)\" message=\"a b\"\n"

	var out bytes.Buffer
	n, err := run(strings.NewReader(in), &out, true)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if n != 1 {
		t.Fatalf("run counted %d events, want 1", n)
	}

	want := "event 1: user_agent,message\n"
	if out.String() != want {
		t.Fatalf("run output = %q, want %q", out.String(), want)
	}
}
