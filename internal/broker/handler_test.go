package broker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filesentry/internal/wire"
)

func TestReadFile(t *testing.T) {
	root := tenant(t)
	writeFile(t, root+"/reports/q1.csv", "region,revenue\n")

	h := &Handler{Policy: testPolicy(t, rule(1001, []string{root + "/**"}, "read", "list"))}
	resp := serveRequest(t, h, 1001, &wire.Request{
		Version: wire.Version,
		Op:      wire.OpRead,
		Path:    root + "/reports/q1.csv",
	})

	if !resp.OK {
		t.Fatalf("read refused: reason=%q error=%q", resp.Reason, resp.Error)
	}
	if string(resp.Data) != "region,revenue\n" {
		t.Fatalf("read returned %q", resp.Data)
	}
}

func TestReadRefusedWithoutGrant(t *testing.T) {
	root := tenant(t)
	writeFile(t, root+"/reports/q1.csv", "x")

	h := &Handler{Policy: testPolicy(t, rule(1001, []string{root + "/**"}, "list"))}
	resp := serveRequest(t, h, 1001, &wire.Request{
		Version: wire.Version,
		Op:      wire.OpRead,
		Path:    root + "/reports/q1.csv",
	})

	if resp.OK {
		t.Fatal("read succeeded without the read op")
	}
	if resp.Reason != wire.ReasonDenied {
		t.Fatalf("reason = %q, want %q", resp.Reason, wire.ReasonDenied)
	}
}

func TestReadMissingFile(t *testing.T) {
	root := tenant(t)

	h := &Handler{Policy: testPolicy(t, rule(1001, []string{root + "/**"}, "read"))}
	resp := serveRequest(t, h, 1001, &wire.Request{
		Version: wire.Version,
		Op:      wire.OpRead,
		Path:    root + "/reports/missing.csv",
	})

	if resp.OK {
		t.Fatal("read of a missing file succeeded")
	}
	if resp.Reason != wire.ReasonNotFound {
		t.Fatalf("reason = %q, want %q", resp.Reason, wire.ReasonNotFound)
	}
}

func TestReadTooLarge(t *testing.T) {
	root := tenant(t)
	writeFile(t, root+"/big.bin", strings.Repeat("x", 4096))

	h := &Handler{
		Policy:  testPolicy(t, rule(1001, []string{root + "/**"}, "read")),
		MaxRead: 1024,
	}
	resp := serveRequest(t, h, 1001, &wire.Request{
		Version: wire.Version,
		Op:      wire.OpRead,
		Path:    root + "/big.bin",
	})

	if resp.OK {
		t.Fatal("read of an oversized file succeeded")
	}
	if resp.Reason != wire.ReasonTooLarge {
		t.Fatalf("reason = %q, want %q", resp.Reason, wire.ReasonTooLarge)
	}
}

func TestWriteFile(t *testing.T) {
	root := tenant(t)
	writeFile(t, root+"/notes.txt", "old")

	h := &Handler{Policy: testPolicy(t, rule(1001, []string{root + "/**"}, "read", "write"))}
	resp := serveRequest(t, h, 1001, &wire.Request{
		Version: wire.Version,
		Op:      wire.OpWrite,
		Path:    root + "/notes.txt",
		Data:    []byte("new contents"),
	})

	if !resp.OK {
		t.Fatalf("write refused: reason=%q error=%q", resp.Reason, resp.Error)
	}

	got, err := os.ReadFile(filepath.FromSlash(root + "/notes.txt"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "new contents" {
		t.Fatalf("file contains %q after write", got)
	}
}

func TestListDirectory(t *testing.T) {
	root := tenant(t)
	writeFile(t, root+"/reports/b.csv", "b")
	writeFile(t, root+"/reports/a.csv", "a")
	if err := os.MkdirAll(filepath.FromSlash(root+"/reports/nested"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	h := &Handler{Policy: testPolicy(t, rule(1001, []string{root + "/**"}, "list"))}
	resp := serveRequest(t, h, 1001, &wire.Request{
		Version: wire.Version,
		Op:      wire.OpList,
		Path:    root + "/reports",
	})

	if !resp.OK {
		t.Fatalf("list refused: reason=%q error=%q", resp.Reason, resp.Error)
	}

	want := []string{"a.csv", "b.csv", "nested"}
	if len(resp.Entries) != len(want) {
		t.Fatalf("list returned %d entries, want %d", len(resp.Entries), len(want))
	}
	for i, name := range want {
		if resp.Entries[i].Name != name {
			t.Fatalf("entry %d = %q, want %q", i, resp.Entries[i].Name, name)
		}
	}
	if !resp.Entries[2].IsDir {
		t.Fatal("nested entry is not marked as a directory")
	}
}

func TestChownRefusedWithoutGrant(t *testing.T) {
	skipWithoutIDs(t)

	root := tenant(t)
	writeFile(t, root+"/notes.txt", "x")

	h := &Handler{Policy: testPolicy(t, rule(1001, []string{root + "/**"}, "read"))}
	resp := serveRequest(t, h, 1001, &wire.Request{
		Version: wire.Version,
		Op:      wire.OpChown,
		Path:    root + "/notes.txt",
		UID:     os.Getuid(),
		GID:     os.Getgid(),
	})

	if resp.OK {
		t.Fatal("chown succeeded without the chown op")
	}
	if resp.Reason != wire.ReasonDenied {
		t.Fatalf("reason = %q, want %q", resp.Reason, wire.ReasonDenied)
	}
}

func TestChownOwnFile(t *testing.T) {
	skipWithoutIDs(t)

	root := tenant(t)
	writeFile(t, root+"/notes.txt", "x")

	h := &Handler{Policy: testPolicy(t, rule(os.Getuid(), []string{root + "/**"}, "chown"))}
	resp := serveRequest(t, h, os.Getuid(), &wire.Request{
		Version: wire.Version,
		Op:      wire.OpChown,
		Path:    root + "/notes.txt",
		UID:     os.Getuid(),
		GID:     os.Getgid(),
	})

	if !resp.OK {
		t.Fatalf("chown of an own in-tenant file refused: reason=%q error=%q", resp.Reason, resp.Error)
	}
}
