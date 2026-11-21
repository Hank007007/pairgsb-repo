package source

import "testing"

func TestNormalizeIgnoresRedundantSegments(t *testing.T) {
	canonical := Normalize("src/main.go")
	for _, in := range []string{"src/./main.go", "src//main.go", "./src/main.go"} {
		if got := Normalize(in); got != canonical {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, canonical)
		}
	}
}

func TestNormalizeEmpty(t *testing.T) {
	if got := Normalize("   "); got != "" {
		t.Fatalf("Normalize of blank input = %q, want empty", got)
	}
}

func TestNormalizeKeepsTrailingName(t *testing.T) {
	for _, in := range []string{"a/b/c.go", "a/b/../b/c.go"} {
		got := Normalize(in)
		if len(got) < 4 || got[len(got)-4:] != "c.go" {
			t.Fatalf("Normalize(%q) = %q, lost the file name", in, got)
		}
	}
}
