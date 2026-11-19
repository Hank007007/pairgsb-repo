package broker

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"filesentry/internal/wire"
)

// TestReadSymlinkEscape covers INC-2024-05-06: a symlink inside the allowed
// tree that resolves outside the rule root must be refused with path_escape
// and must never return bytes of the outside target.
func TestReadSymlinkEscape(t *testing.T) {
	root := tenant(t)
	outside := tenant(t)

	writeFile(t, outside+"/shadow", "secret\n")
	writeFile(t, root+"/inside.txt", "safe\n")
	if err := os.Symlink(
		filepath.FromSlash(outside+"/shadow"),
		filepath.FromSlash(root+"/link"),
	); err != nil {
		t.Skipf("symlink not supported on this host: %v", err)
	}

	h := &Handler{Policy: testPolicy(t, rule(1001, []string{root + "/**"}, "read", "list"))}

	resp := serveRequest(t, h, 1001, &wire.Request{
		Version: wire.Version,
		Op:      wire.OpRead,
		Path:    root + "/link",
	})
	if resp.OK {
		t.Fatal("read through an escaping symlink succeeded")
	}
	if resp.Reason != wire.ReasonPathEscape {
		t.Fatalf("reason = %q, want %q", resp.Reason, wire.ReasonPathEscape)
	}
	if len(resp.Data) != 0 {
		t.Fatal("response leaked bytes of the outside target")
	}

	// A symlink that stays inside the same root behaves as the file it names.
	if err := os.Symlink(
		filepath.FromSlash(root+"/inside.txt"),
		filepath.FromSlash(root+"/local-link"),
	); err != nil {
		t.Skipf("symlink not supported on this host: %v", err)
	}
	ok := serveRequest(t, h, 1001, &wire.Request{
		Version: wire.Version,
		Op:      wire.OpRead,
		Path:    root + "/local-link",
	})
	if !ok.OK || string(ok.Data) != "safe\n" {
		t.Fatalf("in-root symlink read failed: %+v", ok)
	}
}

// TestListSymlinkEscape ensures list containment is enforced as well.
func TestListSymlinkEscape(t *testing.T) {
	root := tenant(t)
	outside := tenant(t)
	if err := os.MkdirAll(filepath.FromSlash(outside+"/dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(
		filepath.FromSlash(outside+"/dir"),
		filepath.FromSlash(root+"/dirlink"),
	); err != nil {
		t.Skipf("symlink not supported on this host: %v", err)
	}

	h := &Handler{Policy: testPolicy(t, rule(1001, []string{root + "/**"}, "list"))}
	resp := serveRequest(t, h, 1001, &wire.Request{
		Version: wire.Version,
		Op:      wire.OpList,
		Path:    root + "/dirlink",
	})
	if resp.OK || resp.Reason != wire.ReasonPathEscape {
		t.Fatalf("escaping list = ok=%v reason=%q", resp.OK, resp.Reason)
	}
}

// TestChownOutsideTenantRoot covers INC-2024-05-21: without a rule covering
// the target, chown must be refused before any system call is attempted.
func TestChownOutsideTenantRoot(t *testing.T) {
	root := tenant(t)
	outside := tenant(t)
	writeFile(t, outside+"/agent-v3.tar", "blob")

	h := &Handler{Policy: testPolicy(t, rule(1001, []string{root + "/**"}, "chown"))}
	resp := serveRequest(t, h, 1001, &wire.Request{
		Version: wire.Version,
		Op:      wire.OpChown,
		Path:    outside + "/agent-v3.tar",
		UID:     1001,
		GID:     1001,
	})
	if resp.OK || resp.Reason != wire.ReasonDenied {
		t.Fatalf("out-of-tenant chown = ok=%v reason=%q", resp.OK, resp.Reason)
	}
}

// TestChownRejectsForeignIDs ensures a caller cannot hand an object to a
// third party, including an invented numeric uid.
func TestChownRejectsForeignIDs(t *testing.T) {
	root := tenant(t)
	writeFile(t, root+"/notes.txt", "x")

	h := &Handler{Policy: testPolicy(t, rule(1001, []string{root + "/**"}, "chown"))}
	for _, ids := range [][2]int{
		{4242, -1},
		{-1, 4242},
		{0, 0},
		{1002, 1001},
	} {
		resp := serveRequest(t, h, 1001, &wire.Request{
			Version: wire.Version,
			Op:      wire.OpChown,
			Path:    root + "/notes.txt",
			UID:     ids[0],
			GID:     ids[1],
		})
		if resp.OK || resp.Reason != wire.ReasonDenied {
			t.Fatalf("chown to %v = ok=%v reason=%q", ids, resp.OK, resp.Reason)
		}
	}

	// The caller's own ids and negative "leave unchanged" values pass policy.
	resp := serveRequest(t, h, 1001, &wire.Request{
		Version: wire.Version,
		Op:      wire.OpChown,
		Path:    root + "/notes.txt",
		UID:     1001,
		GID:     -1,
	})
	if resp.OK {
		return
	}
	if resp.Reason != wire.ReasonIOError {
		t.Fatalf("own-id chown refused with reason=%q error=%q", resp.Reason, resp.Error)
	}
}

// TestPeerIdentityIgnoresEnvironment covers INC-2024-03-11: the daemon
// startup environment must never supply a caller identity. The kernel
// credential is the only accepted evidence.
func TestPeerIdentityIgnoresEnvironment(t *testing.T) {
	t.Setenv("FILESENTRY_UID", "1001")

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	if _, err := peerIdentity(server); err == nil {
		t.Fatal("peerIdentity accepted an environment-derived identity")
	}
}
