package tuple

import "testing"

func TestParseSubject(t *testing.T) {
	tests := []struct {
		input string
		want  Subject
	}{
		{"user:alice", Subject{Namespace: "user", ID: "alice"}},
		{"group:eng#member", Subject{Namespace: "group", ID: "eng", Relation: "member"}},
		{"user:*", Subject{Namespace: "user", ID: "*"}},
		{"service:ci-bot", Subject{Namespace: "service", ID: "ci-bot"}},
	}
	for _, tc := range tests {
		got, err := ParseSubject(tc.input)
		if err != nil {
			t.Fatalf("ParseSubject(%q): %v", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("ParseSubject(%q) = %+v, want %+v", tc.input, got, tc.want)
		}
		if roundTrip := got.String(); roundTrip != tc.input {
			t.Errorf("round trip of %q = %q", tc.input, roundTrip)
		}
	}
}

func TestParseSubjectRejects(t *testing.T) {
	bad := []string{
		"",
		"alice",
		"user:",
		"user:alice#",
		"user:alice#member#owner",
		"user:*#member",
	}
	for _, input := range bad {
		if _, err := ParseSubject(input); err == nil {
			t.Errorf("ParseSubject(%q) succeeded, want error", input)
		}
	}
}

func TestSubjectClassification(t *testing.T) {
	terminal, _ := ParseSubject("user:alice")
	userset, _ := ParseSubject("group:eng#member")
	wildcard, _ := ParseSubject("user:*")

	if !terminal.IsTerminal() || terminal.IsUserset() || terminal.IsWildcard() {
		t.Errorf("terminal subject misclassified: %+v", terminal)
	}
	if userset.IsTerminal() || !userset.IsUserset() || userset.IsWildcard() {
		t.Errorf("subject set misclassified: %+v", userset)
	}
	if !wildcard.IsWildcard() {
		t.Errorf("wildcard subject misclassified: %+v", wildcard)
	}
}

func TestSubjectObject(t *testing.T) {
	obj, ok := Subject{Namespace: "group", ID: "eng", Relation: "member"}.Object()
	if !ok || obj != (Object{Namespace: "group", ID: "eng"}) {
		t.Errorf("Object() = %+v, %v", obj, ok)
	}
	if _, ok := (Subject{Namespace: "user", ID: "*"}).Object(); ok {
		t.Errorf("wildcard subject unexpectedly resolved to an object")
	}
}
