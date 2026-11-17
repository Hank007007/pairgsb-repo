package parse

import (
	"strings"
	"testing"

	"sudopolicy/internal/policy"
)

const samplePolicy = `
# Operations may look at files on the web hosts.
ops,root web1 = (root) NOPASSWD: /bin/ls

deploy web1,web2 = (www-data) SETENV: /usr/bin/rsync --archive
`

func TestParseSamplePolicy(t *testing.T) {
	table, err := Parse(strings.NewReader(samplePolicy))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := len(table.Entries), 2; got != want {
		t.Fatalf("entries = %d, want %d", got, want)
	}

	first := table.Entries[0]
	if first.Line != 3 {
		t.Errorf("first line = %d, want 3", first.Line)
	}
	if !first.HasTag(policy.TagNoPasswd) {
		t.Errorf("first entry tags = %v, want NOPASSWD", first.Tags)
	}
	if first.Command != "/bin/ls" {
		t.Errorf("first command = %q, want /bin/ls", first.Command)
	}
	if len(first.Args) != 0 {
		t.Errorf("first args = %v, want none", first.Args)
	}
	if got, want := strings.Join(first.Users, ","), "ops,root"; got != want {
		t.Errorf("first users = %q, want %q", got, want)
	}
	if got, want := strings.Join(first.Runas, ","), "root"; got != want {
		t.Errorf("first runas = %q, want %q", got, want)
	}

	second := table.Entries[1]
	if !second.HasTag(policy.TagSetenv) {
		t.Errorf("second entry tags = %v, want SETENV", second.Tags)
	}
	if got, want := strings.Join(second.Hosts, ","), "web1,web2"; got != want {
		t.Errorf("second hosts = %q, want %q", got, want)
	}
	if got, want := strings.Join(second.Args, " "), "--archive"; got != want {
		t.Errorf("second args = %q, want %q", got, want)
	}
}

func TestParseCommentsAndBlankLines(t *testing.T) {
	table, err := Parse(strings.NewReader("\n# comment\n\n   \n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(table.Entries) != 0 {
		t.Fatalf("entries = %d, want 0", len(table.Entries))
	}
}

func TestParseRejectsMalformedLine(t *testing.T) {
	if _, err := Parse(strings.NewReader("nonsense\n")); err == nil {
		t.Fatal("Parse accepted a line without '='")
	}
}

func TestParseRejectsRelativeCommand(t *testing.T) {
	_, err := Parse(strings.NewReader("bob host = (root) ls\n"))
	if err == nil {
		t.Fatal("Parse accepted a relative command path")
	}
}
