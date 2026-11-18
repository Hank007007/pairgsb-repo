package pathguard

import (
	"path/filepath"
	"testing"
)

func TestResolvePlacesNamesUnderRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs", "abc", "in")

	cases := []string{
		"README.md",
		"src/main.go",
		"src/vendor/lib/lib.go",
		"a/b/c/d.txt",
	}
	for _, name := range cases {
		target, err := Resolve(root, name)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", name, err)
		}
		want := filepath.Join(root, filepath.FromSlash(name))
		if target != want {
			t.Fatalf("Resolve(%q) = %q, want %q", name, target, want)
		}
	}
}

func TestCheckNameAcceptsOrdinaryNames(t *testing.T) {
	for _, name := range []string{"a", "a/b", "a-b_c.txt", "dir/"} {
		if err := CheckName(name); err != nil {
			t.Fatalf("CheckName(%q): %v", name, err)
		}
	}
}

func TestCheckNameRejectsLeadingSlash(t *testing.T) {
	for _, name := range []string{"/etc/passwd", "/tmp/x"} {
		if err := CheckName(name); err == nil {
			t.Fatalf("CheckName(%q) accepted an absolute name", name)
		}
	}
}
