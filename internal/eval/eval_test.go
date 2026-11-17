package eval

import (
	"os"
	"path/filepath"
	"testing"

	"sudopolicy/internal/policy"
)

func TestEvaluateAllowsMatchingRule(t *testing.T) {
	command := tempExecutable(t)
	table := &policy.Table{Entries: []policy.Entry{{
		Line:    1,
		Users:   []string{"bob"},
		Hosts:   []string{"web1"},
		Runas:   []string{"root"},
		Tags:    []string{policy.TagNoPasswd},
		Command: command,
	}}}

	result, err := Evaluate(table, policy.Request{
		User: "bob",
		Host: "web1",
		Argv: []string{command},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("Allowed = false, reason %q", result.Reason)
	}
	if result.RunAs != "root" {
		t.Errorf("RunAs = %q, want root", result.RunAs)
	}
	if result.RuleLine != 1 {
		t.Errorf("RuleLine = %d, want 1", result.RuleLine)
	}
}

func TestEvaluateDeniesUnknownUser(t *testing.T) {
	command := tempExecutable(t)
	table := &policy.Table{Entries: []policy.Entry{{
		Line:    1,
		Users:   []string{"bob"},
		Hosts:   []string{"web1"},
		Runas:   []string{"root"},
		Tags:    []string{policy.TagNoPasswd},
		Command: command,
	}}}

	result, err := Evaluate(table, policy.Request{
		User: "alice",
		Host: "web1",
		Argv: []string{command},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.Allowed {
		t.Fatal("Allowed = true for a user no rule names")
	}
	if result.RunAs != "" {
		t.Errorf("RunAs = %q, want empty", result.RunAs)
	}
}

func tempExecutable(t *testing.T) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "cmd")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	file.Close()
	path, err := filepath.Abs(file.Name())
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	return path
}
