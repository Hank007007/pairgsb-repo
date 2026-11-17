package match

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUserLiteral(t *testing.T) {
	if !User("bob", "bob", nil) {
		t.Error("literal user did not match itself")
	}
	if User("bob", "alice", nil) {
		t.Error("literal user matched a different name")
	}
}

func TestUserWildcard(t *testing.T) {
	if !User("*", "anyone", nil) {
		t.Error("wildcard did not match")
	}
}

func TestUserGroup(t *testing.T) {
	if !User("%ops", "bob", []string{"ops", "wheel"}) {
		t.Error("group pattern did not match a member")
	}
	if User("%ops", "bob", []string{"wheel"}) {
		t.Error("group pattern matched a non-member")
	}
}

func TestHost(t *testing.T) {
	if !Host("*", "web1") {
		t.Error("wildcard host did not match")
	}
	if !Host("web1", "web1") {
		t.Error("host did not match itself")
	}
	if Host("web1", "web2") {
		t.Error("host matched a different name")
	}
}

func TestCommandSamePath(t *testing.T) {
	path := tempExecutable(t)
	if !Command(path, path) {
		t.Errorf("Command(%q, %q) = false, want true", path, path)
	}
}

func TestCommandDifferentNames(t *testing.T) {
	pattern := filepath.Join(t.TempDir(), "systemctl")
	argv0 := filepath.Join(t.TempDir(), "other")
	if Command(pattern, argv0) {
		t.Errorf("Command(%q, %q) = true, want false", pattern, argv0)
	}
}

func TestArgsLiteral(t *testing.T) {
	if !Args([]string{"restart", "nginx"}, []string{"restart", "nginx"}) {
		t.Error("literal arguments did not match")
	}
	if Args([]string{"restart", "nginx"}, []string{"restart", "apache"}) {
		t.Error("literal arguments matched different values")
	}
}

// tempExecutable creates an empty file and returns its absolute path. Using a
// real file keeps path comparisons independent of the host's directory layout.
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
