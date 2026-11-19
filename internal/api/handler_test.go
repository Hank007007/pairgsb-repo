package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pathvault/internal/audit"
	"pathvault/internal/lease"
	"pathvault/internal/policy"
	"pathvault/internal/store"
	"pathvault/internal/token"
)

const testAdminToken = "adm_test"

type fixture struct {
	srv   *httptest.Server
	audit *bytes.Buffer
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	kv := store.New()
	if err := kv.LoadFile("../../testdata/secrets.json"); err != nil {
		t.Fatalf("load secrets: %v", err)
	}

	var accessors []policy.Accessor
	for _, f := range []string{
		"../../testdata/policy-payments.json",
		"../../testdata/policy-ops.json",
	} {
		a, err := policy.Load(f)
		if err != nil {
			t.Fatalf("load policy %s: %v", f, err)
		}
		accessors = append(accessors, a...)
	}
	pol, err := policy.New(accessors)
	if err != nil {
		t.Fatalf("build policy: %v", err)
	}

	buf := &bytes.Buffer{}
	api := New(pol, kv, lease.NewManager(), token.NewManager(), audit.NewLogger(buf), testAdminToken)
	ts := httptest.NewServer(api.Handler())
	t.Cleanup(ts.Close)
	return &fixture{srv: ts, audit: buf}
}

func (f *fixture) get(t *testing.T, path, accessorToken, leaseID string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, f.srv.URL+"/v1/secret/"+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if accessorToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessorToken)
	}
	if leaseID != "" {
		req.Header.Set("X-Lease-Id", leaseID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func (f *fixture) post(t *testing.T, path, accessorToken string, body any) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, f.srv.URL+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if accessorToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessorToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func decodeBody(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func TestHealthz(t *testing.T) {
	f := newFixture(t)
	resp, err := http.Get(f.srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("get healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestReadAllowed(t *testing.T) {
	f := newFixture(t)
	resp := f.get(t, "kv/app/payments/stripe", "acc_payments_dev", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body secretBody
	decodeBody(t, resp, &body)
	if body.Path != "kv/app/payments/stripe" {
		t.Errorf("path = %q", body.Path)
	}
	if body.Value != "sk_live_stripe_8f2c1a" {
		t.Errorf("value = %q", body.Value)
	}
	if body.Backend != "payments-core" {
		t.Errorf("backend = %q", body.Backend)
	}
	if body.LeaseID == "" {
		t.Error("expected a lease id")
	}
}

func TestReadRequiresAccessorToken(t *testing.T) {
	f := newFixture(t)
	if resp := f.get(t, "kv/app/payments/stripe", "", ""); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if resp := f.get(t, "kv/app/payments/stripe", "acc_bogus", ""); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestReadPolicyDenialIsNotFound(t *testing.T) {
	f := newFixture(t)
	// svc-payments has no rule covering kv/app/db/...
	resp := f.get(t, "kv/app/db/prod/password", "acc_payments_dev", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestReadMissingSecretIsNotFound(t *testing.T) {
	f := newFixture(t)
	resp := f.get(t, "kv/app/payments/does-not-exist", "acc_payments_dev", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestLeaseReadRepeats(t *testing.T) {
	f := newFixture(t)
	first := f.get(t, "kv/app/payments/stripe", "acc_payments_dev", "")
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first read status = %d", first.StatusCode)
	}
	var body secretBody
	decodeBody(t, first, &body)

	again := f.get(t, "kv/app/payments/stripe", "acc_payments_dev", body.LeaseID)
	if again.StatusCode != http.StatusOK {
		t.Fatalf("lease read status = %d, want 200", again.StatusCode)
	}
	var second secretBody
	decodeBody(t, again, &second)
	if second.LeaseID != body.LeaseID {
		t.Errorf("lease id changed: %q -> %q", body.LeaseID, second.LeaseID)
	}
	if second.Value != body.Value {
		t.Errorf("value changed across lease read")
	}
}

func TestLeaseReadRejectsOtherAccessor(t *testing.T) {
	f := newFixture(t)
	first := f.get(t, "kv/app/payments/stripe", "acc_payments_dev", "")
	var body secretBody
	decodeBody(t, first, &body)

	resp := f.get(t, "kv/app/payments/stripe", "acc_audit_dev", body.LeaseID)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestWrapAndUnwrap(t *testing.T) {
	f := newFixture(t)
	wrapResp := f.post(t, "/v1/wrap", "acc_ops_dev", map[string]any{
		"path": "kv/app/ops/deploy-key",
	})
	if wrapResp.StatusCode != http.StatusCreated {
		t.Fatalf("wrap status = %d, want 201", wrapResp.StatusCode)
	}
	var wrap wrapBody
	decodeBody(t, wrapResp, &wrap)
	if !strings.HasPrefix(wrap.Token, "wrap_") {
		t.Fatalf("wrap token = %q", wrap.Token)
	}
	if wrap.ExpiresAt == "" {
		t.Fatal("expected an expiry")
	}

	unwrapResp := f.post(t, "/v1/unwrap", "", map[string]any{"token": wrap.Token})
	if unwrapResp.StatusCode != http.StatusOK {
		t.Fatalf("unwrap status = %d, want 200", unwrapResp.StatusCode)
	}
	var secret secretBody
	decodeBody(t, unwrapResp, &secret)
	if secret.Path != "kv/app/ops/deploy-key" {
		t.Errorf("path = %q", secret.Path)
	}
	if secret.Value != "ops-deploy-91cd7f" {
		t.Errorf("value = %q", secret.Value)
	}
	if secret.LeaseID == "" {
		t.Error("expected unwrap to create a lease")
	}
}

func TestWrapDeniedPath(t *testing.T) {
	f := newFixture(t)
	resp := f.post(t, "/v1/wrap", "acc_payments_dev", map[string]any{
		"path": "kv/app/db/prod/password",
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestWrapNeedsAccessorToken(t *testing.T) {
	f := newFixture(t)
	resp := f.post(t, "/v1/wrap", "", map[string]any{"path": "kv/app/payments/stripe"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestUnwrapUnknownToken(t *testing.T) {
	f := newFixture(t)
	resp := f.post(t, "/v1/unwrap", "", map[string]any{"token": "wrap_unknown"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestRevokeRequiresAdminToken(t *testing.T) {
	f := newFixture(t)
	resp := f.post(t, "/v1/backends/payments-core/revoke", "acc_ops_dev", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestRevokeUnknownBackend(t *testing.T) {
	f := newFixture(t)
	resp := f.post(t, "/v1/backends/no-such-backend/revoke", testAdminToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestRevokeBackendInvalidatesLease(t *testing.T) {
	f := newFixture(t)
	read := f.get(t, "kv/app/payments/stripe", "acc_payments_dev", "")
	if read.StatusCode != http.StatusOK {
		t.Fatalf("read status = %d", read.StatusCode)
	}
	var body secretBody
	decodeBody(t, read, &body)

	revoke := f.post(t, "/v1/backends/payments-core/revoke", testAdminToken, nil)
	if revoke.StatusCode != http.StatusOK {
		t.Fatalf("revoke status = %d, want 200", revoke.StatusCode)
	}
	var revoked revokeBody
	decodeBody(t, revoke, &revoked)
	if revoked.Backend != "payments-core" || revoked.Revoked != 1 {
		t.Fatalf("revoke body = %+v", revoked)
	}

	again := f.get(t, "kv/app/payments/stripe", "acc_payments_dev", body.LeaseID)
	if again.StatusCode != http.StatusForbidden {
		t.Fatalf("lease read after revoke = %d, want 403", again.StatusCode)
	}
}

func TestRevokeLeavesOtherBackendsAlone(t *testing.T) {
	f := newFixture(t)
	ops := f.get(t, "kv/app/ops/deploy-key", "acc_ops_dev", "")
	var opsBody secretBody
	decodeBody(t, ops, &opsBody)

	revoke := f.post(t, "/v1/backends/payments-core/revoke", testAdminToken, nil)
	if revoke.StatusCode != http.StatusOK {
		t.Fatalf("revoke status = %d", revoke.StatusCode)
	}

	resp := f.get(t, "kv/app/ops/deploy-key", "acc_ops_dev", opsBody.LeaseID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unrelated backend lease read = %d, want 200", resp.StatusCode)
	}
}

func TestAuditRecordsReadByAccessor(t *testing.T) {
	f := newFixture(t)
	if resp := f.get(t, "kv/app/payments/stripe", "acc_payments_dev", ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("read status = %d", resp.StatusCode)
	}

	var found bool
	for _, line := range strings.Split(strings.TrimSpace(f.audit.String()), "\n") {
		if line == "" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("decode audit line %q: %v", line, err)
		}
		if ev["accessor"] == "svc-payments" &&
			ev["path"] == "kv/app/payments/stripe" &&
			ev["action"] == "read" &&
			ev["decision"] == "allow" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no allow read event for acc_payments_dev; log:\n%s", f.audit.String())
	}
}
