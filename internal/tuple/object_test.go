package tuple

import "testing"

func TestParseObject(t *testing.T) {
	tests := []struct {
		input string
		want  Object
	}{
		{"document:doc-1", Object{Namespace: "document", ID: "doc-1"}},
		{"folder:eng/2024", Object{Namespace: "folder", ID: "eng/2024"}},
		{"_svc:worker.1", Object{Namespace: "_svc", ID: "worker.1"}},
	}
	for _, tc := range tests {
		got, err := ParseObject(tc.input)
		if err != nil {
			t.Fatalf("ParseObject(%q): %v", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("ParseObject(%q) = %+v, want %+v", tc.input, got, tc.want)
		}
	}
}

func TestParseObjectRejects(t *testing.T) {
	bad := []string{
		"",
		"document",
		":doc-1",
		"9document:doc-1",
		"document:",
		"document:#viewer",
		"document:doc@user",
	}
	for _, input := range bad {
		if _, err := ParseObject(input); err == nil {
			t.Errorf("ParseObject(%q) succeeded, want error", input)
		}
	}
}

func TestObjectString(t *testing.T) {
	obj := Object{Namespace: "document", ID: "doc-1"}
	if got := obj.String(); got != "document:doc-1" {
		t.Errorf("String() = %q", got)
	}
}
