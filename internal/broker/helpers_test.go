package broker

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"

	"filesentry/internal/acl"
	"filesentry/internal/wire"
)

// testPolicy builds a policy from rules, failing the test if it is rejected.
func testPolicy(t *testing.T, rules ...*acl.Rule) *acl.Policy {
	t.Helper()

	raw, err := json.Marshal(struct {
		Rules []*acl.Rule `json:"rules"`
	}{Rules: rules})
	if err != nil {
		t.Fatalf("encode policy: %v", err)
	}

	p, err := acl.Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	return p
}

func rule(uid int, paths []string, ops ...string) *acl.Rule {
	return &acl.Rule{UID: uid, Paths: paths, Ops: ops}
}

// tenant creates a temporary directory and returns it in the normalised slash
// form the policy uses.
func tenant(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "filesentry")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}
	normal, err := acl.NormalizePath(root)
	if err != nil {
		t.Fatalf("normalize temp dir: %v", err)
	}
	return normal
}

func writeFile(t *testing.T, name, content string) {
	t.Helper()

	native := filepath.FromSlash(name)
	if err := os.MkdirAll(filepath.Dir(native), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(native), err)
	}
	if err := os.WriteFile(native, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// serveRequest runs one request through the handler as uid.
func serveRequest(t *testing.T, h *Handler, uid int, req *wire.Request) *wire.Response {
	t.Helper()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("encode request: %v", err)
	}
	raw = append(raw, '\n')

	go func() {
		client.Write(raw)
		client.Close()
	}()

	h.Identity = func(net.Conn) (int, error) { return uid, nil }
	return h.serve(server)
}

// skipWithoutIDs skips a test whose setup needs a meaningful uid.
func skipWithoutIDs(t *testing.T) {
	t.Helper()

	if os.Getuid() < 0 {
		t.Skip("this platform does not report a numeric uid")
	}
}
