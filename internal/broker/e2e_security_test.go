package broker

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"filesentry/internal/acl"
	"filesentry/internal/wire"
)

// e2ePolicy builds the production-shaped policy from the incident report
// against a throwaway tenant tree.
func e2ePolicy(t *testing.T, base string) *acl.Policy {
	t.Helper()
	doc := map[string]any{
		"rules": []map[string]any{
			{"uid": 1001, "paths": []string{base + "/srv/tenants/acme/**"}, "ops": []string{"read", "list", "write", "chown"}},
			{"uid": 1001, "paths": []string{base + "/srv/tenants/acme/private/**"}, "ops": []string{"deny"}},
			{"uid": 1001, "paths": []string{base + "/srv/tenants/*/reports"}, "ops": []string{"read"}},
			{"uid": 1002, "paths": []string{base + "/srv/tenants/beta/**"}, "ops": []string{"read", "list", "write", "chown"}},
		},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	p, err := acl.Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse e2e policy: %v", err)
	}
	return p
}

func TestIncidentsEndToEnd(t *testing.T) {
	base := tenant(t)
	writeFile(t, base+"/srv/tenants/acme/reports/q1.csv", "region,revenue\n")
	writeFile(t, base+"/srv/tenants/acme/private/keys.txt", "TOPSECRET")
	writeFile(t, base+"/srv/tenants/acme/private/deeper/reports", "DEEP")
	writeFile(t, base+"/srv/tenants/acme-evil/x", "EVIL")
	writeFile(t, base+"/srv/packages/agent-v3.tar", "BLOB")
	writeFile(t, base+"/srv/tenants/beta/secret.txt", "BETA")

	h := &Handler{Policy: e2ePolicy(t, base)}
	call := func(uid int, req *wire.Request) *wire.Response {
		return serveRequest(t, h, uid, req)
	}

	// INC-2024-04-02: acme-evil has no rule and must not match acme.
	resp := call(1001, &wire.Request{Version: 1, Op: wire.OpList, Path: base + "/srv/tenants/acme-evil"})
	if resp.OK || resp.Reason != wire.ReasonDenied {
		t.Fatalf("acme-evil list = ok=%v reason=%q", resp.OK, resp.Reason)
	}

	// "*" is one segment: the deep private path must not hit the reports rule.
	resp = call(1001, &wire.Request{Version: 1, Op: wire.OpRead, Path: base + "/srv/tenants/acme/private/deeper/reports"})
	if resp.OK || resp.Reason != wire.ReasonDenied {
		t.Fatalf("deep star match = ok=%v reason=%q", resp.OK, resp.Reason)
	}

	// The legitimate one-segment match still works.
	resp = call(1001, &wire.Request{Version: 1, Op: wire.OpRead, Path: base + "/srv/tenants/acme/reports/q1.csv"})
	if !resp.OK || string(resp.Data) != "region,revenue\n" {
		t.Fatalf("legitimate read failed: %+v", resp)
	}

	// INC-2024-04-19: the private deny wins repeatedly and deterministically.
	for i := 0; i < 100; i++ {
		resp = call(1001, &wire.Request{Version: 1, Op: wire.OpRead, Path: base + "/srv/tenants/acme/private/keys.txt"})
		if resp.OK || resp.Reason != wire.ReasonDenied {
			t.Fatalf("iteration %d: private read = ok=%v reason=%q", i, resp.OK, resp.Reason)
		}
	}

	// INC-2024-05-21: no rule covers /srv/packages, so chown is refused.
	resp = call(1001, &wire.Request{Version: 1, Op: wire.OpChown, Path: base + "/srv/packages/agent-v3.tar", UID: 4242, GID: -1})
	if resp.OK || resp.Reason != wire.ReasonDenied {
		t.Fatalf("packages chown = ok=%v reason=%q", resp.OK, resp.Reason)
	}

	// INC-2024-03-11: uid 1002 must not reach acme even though it has its own tenant.
	resp = call(1002, &wire.Request{Version: 1, Op: wire.OpRead, Path: base + "/srv/tenants/acme/reports/q1.csv"})
	if resp.OK || resp.Reason != wire.ReasonDenied {
		t.Fatalf("cross-tenant read = ok=%v reason=%q", resp.OK, resp.Reason)
	}

	// Lexical ".." is resolved before matching and cannot keep a request
	// inside the acme pattern.
	resp = call(1001, &wire.Request{Version: 1, Op: wire.OpRead, Path: base + "/srv/tenants/acme/../beta/secret.txt"})
	if resp.OK || resp.Reason != wire.ReasonDenied {
		t.Fatalf("dot-dot traversal = ok=%v reason=%q", resp.OK, resp.Reason)
	}

	// A trailing slash or repeated slashes selects the same rule.
	listResp := call(1001, &wire.Request{Version: 1, Op: wire.OpList, Path: base + "//srv/tenants/acme/reports/"})
	plain := call(1001, &wire.Request{Version: 1, Op: wire.OpList, Path: base + "/srv/tenants/acme/reports"})
	if !listResp.OK || !plain.OK || len(listResp.Entries) != len(plain.Entries) {
		t.Fatalf("normalised path behaves differently: %+v vs %+v", listResp, plain)
	}
}

// TestSymlinkIncidentEndToEnd reproduces INC-2024-05-06 against the real
// resolver with the production policy shape.
func TestSymlinkIncidentEndToEnd(t *testing.T) {
	base := tenant(t)
	outside := tenant(t)
	writeFile(t, base+"/srv/tenants/acme/archive/real.txt", "in-tenant\n")
	writeFile(t, outside+"/shadow", "outside-secret\n")

	if err := os.MkdirAll(filepath.FromSlash(base+"/srv/tenants/acme"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(
		filepath.FromSlash(outside+"/shadow"),
		filepath.FromSlash(base+"/srv/tenants/acme/link"),
	); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	h := &Handler{Policy: e2ePolicy(t, base)}
	resp := serveRequest(t, h, 1001, &wire.Request{
		Version: 1,
		Op:      wire.OpRead,
		Path:    base + "/srv/tenants/acme/link",
	})
	if resp.OK || resp.Reason != wire.ReasonPathEscape {
		t.Fatalf("escaping symlink = ok=%v reason=%q", resp.OK, resp.Reason)
	}
	if len(resp.Data) != 0 {
		t.Fatal("outside bytes were returned")
	}
}
