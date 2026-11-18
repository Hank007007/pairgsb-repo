package pathguard

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pairgsb/unpackfence/internal/fault"
)

func TestCheckNameRejectsPlatformAbsoluteForms(t *testing.T) {
	cases := []string{
		"/etc/passwd",
		`\etc\passwd`,
		`C:\x\y`,
		"C:/x/y",
		"c:relative",
		`\\host\share\z`,
		"//host/share/z",
	}
	for _, name := range cases {
		if err := CheckName(name); !errors.Is(err, fault.ErrUnsafeName) {
			t.Fatalf("CheckName(%q) = %v, want unsafe name", name, err)
		}
	}
}

func TestCheckNameTreatsBackslashAsSeparator(t *testing.T) {
	root := filepath.Join(t.TempDir(), "in")
	target, err := Resolve(root, `a\b\c.txt`)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := filepath.Join(root, "a", "b", "c.txt")
	if target != want {
		t.Fatalf("Resolve = %q, want %q", target, want)
	}
}

func TestResolveRejectsSegmentEscape(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jobs", "abc", "in")
	cases := []string{
		"../../abcdef/in/x",
		`..\..\abcdef\in\x`,
		"a/../../x",
		`a\..\..\..\x`,
	}
	for _, name := range cases {
		if _, err := Resolve(root, name); !errors.Is(err, fault.ErrOutsideRoot) {
			t.Fatalf("Resolve(%q) = %v, want outside root", name, err)
		}
	}
}

func TestWithinUsesSegments(t *testing.T) {
	root := filepath.Join("jobs", "abc")
	if Within(root, filepath.Join("jobs", "abcdef", "in", "x")) {
		t.Fatal("sibling with matching string prefix counted as inside")
	}
	if !Within(root, filepath.Join(root, "in", "x")) {
		t.Fatal("child counted as outside")
	}
}

func TestCheckSymlinkTargetRejectsEscapes(t *testing.T) {
	root := filepath.Join(t.TempDir(), "in")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	cases := []string{
		"/tmp/outside",
		`C:\outside`,
		"c:relative",
		`\\host\share\z`,
		"//host/share/z",
		"../../outside",
		`..\..\outside`,
	}
	for _, target := range cases {
		if err := CheckSymlinkTarget(root, link, target); !errors.Is(err, fault.ErrSymlinkEscape) {
			t.Fatalf("CheckSymlinkTarget(%q) = %v, want symlink escape", target, err)
		}
	}
}

func TestCheckSymlinkTargetAllowsInsideTargets(t *testing.T) {
	root := filepath.Join(t.TempDir(), "in")
	if err := os.MkdirAll(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "dir", "link")
	for _, target := range []string{"file.txt", "../file.txt", "sub/file.txt", `sub\file.txt`} {
		if err := CheckSymlinkTarget(root, link, target); err != nil {
			t.Fatalf("CheckSymlinkTarget(%q): %v", target, err)
		}
	}
}

func TestResolveExistingRejectsEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink creation is not available: %v", err)
	}

	if _, err := ResolveExisting(root, filepath.Join(link, "payload")); !errors.Is(err, fault.ErrSymlinkEscape) {
		t.Fatalf("ResolveExisting = %v, want symlink escape", err)
	}
}

func TestResolveExistingFollowsInsideSymlink(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink creation is not available: %v", err)
	}

	got, err := ResolveExisting(root, filepath.Join(link, "payload"))
	if err != nil {
		t.Fatalf("ResolveExisting: %v", err)
	}
	want := filepath.Join(real, "payload")
	if got != want {
		t.Fatalf("ResolveExisting = %q, want %q", got, want)
	}
}
