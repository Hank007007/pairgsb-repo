package anchor

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"beaconproof/internal/chain"
	"beaconproof/internal/receipt"
	"beaconproof/internal/tsa"
)

type harness struct {
	t     *testing.T
	auth  *tsa.Authority
	srv   *httptest.Server
	store *chain.Store
	dir   *receipt.Dir
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	auth, err := tsa.NewAuthority()
	if err != nil {
		t.Fatalf("authority: %v", err)
	}
	srv := httptest.NewServer(auth.Handler())
	t.Cleanup(srv.Close)

	root := t.TempDir()
	store, err := chain.Open(filepath.Join(root, "audit.log"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	dir, err := receipt.OpenDir(filepath.Join(root, "receipts"))
	if err != nil {
		t.Fatalf("open receipts: %v", err)
	}
	return &harness{t: t, auth: auth, srv: srv, store: store, dir: dir}
}

func (h *harness) appendN(n int) {
	h.t.Helper()
	for i := 0; i < n; i++ {
		if _, err := h.store.Append("alice", "read", fmt.Sprintf("doc-%d", i)); err != nil {
			h.t.Fatalf("append: %v", err)
		}
	}
}

func (h *harness) submitter() *Submitter {
	return NewSubmitter(h.store, h.dir, tsa.NewClient(h.srv.URL, nil), h.auth.PublicKey(), h.srv.URL)
}

func (h *harness) runner(p Policy) *Runner {
	h.t.Helper()
	r, err := NewRunner(p, h.submitter(), h.dir)
	if err != nil {
		h.t.Fatalf("runner: %v", err)
	}
	return r
}

func TestSubmitStoresVerifiableReceipt(t *testing.T) {
	h := newHarness(t)
	h.appendN(8)

	rec, err := h.submitter().Submit(context.Background(), 1, 8)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if rec.FromSeq != 1 || rec.ToSeq != 8 {
		t.Fatalf("receipt covers [%d, %d], want [1, 8]", rec.FromSeq, rec.ToSeq)
	}

	stored, err := h.dir.LoadAll()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("stored %d receipts, want 1", len(stored))
	}
	if stored[0].Root != rec.Root {
		t.Fatal("stored root differs from returned root")
	}

	tok := tsa.Token{RootHex: rec.Root, GenTime: rec.GenTime, Sig: rec.Sig}
	if err := tsa.Verify(tok, h.auth.PublicKey()); err != nil {
		t.Fatalf("signature does not verify: %v", err)
	}
	sig, err := base64.StdEncoding.DecodeString(rec.Sig)
	if err != nil {
		t.Fatalf("signature is not base64: %v", err)
	}
	if len(sig) != ed25519.SignatureSize {
		t.Fatalf("signature has %d bytes", len(sig))
	}
}

func TestRunnerAnchorsWhenIntervalReached(t *testing.T) {
	h := newHarness(t)
	h.appendN(4)

	r := h.runner(Policy{Interval: 4})
	dec, err := r.Tick(context.Background(), h.store.HeadSeq())
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if !dec.Anchor {
		t.Fatalf("policy did not anchor: %s", dec.Reason)
	}
	if dec.FromSeq != 1 || dec.ToSeq != 4 {
		t.Fatalf("anchored [%d, %d], want [1, 4]", dec.FromSeq, dec.ToSeq)
	}
	if got := r.AnchoredSeq(); got != 4 {
		t.Fatalf("anchored seq is %d, want 4", got)
	}
}

func TestRunnerDoesNothingBelowInterval(t *testing.T) {
	h := newHarness(t)
	h.appendN(3)

	r := h.runner(Policy{Interval: 10})
	dec, err := r.Tick(context.Background(), h.store.HeadSeq())
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if dec.Anchor {
		t.Fatal("policy anchored below the interval")
	}
	stored, _ := h.dir.LoadAll()
	if len(stored) != 0 {
		t.Fatalf("stored %d receipts, want 0", len(stored))
	}
}

func TestRunnerRecoversAnchoredPositionOnRestart(t *testing.T) {
	h := newHarness(t)
	h.appendN(6)

	r := h.runner(Policy{Interval: 6})
	if _, err := r.Tick(context.Background(), h.store.HeadSeq()); err != nil {
		t.Fatalf("tick: %v", err)
	}

	again := h.runner(Policy{Interval: 6})
	if got := again.AnchoredSeq(); got != 6 {
		t.Fatalf("restarted runner reports anchored seq %d, want 6", got)
	}
}

func TestSubmitRejectsInvertedInterval(t *testing.T) {
	h := newHarness(t)
	h.appendN(2)
	if _, err := h.submitter().Submit(context.Background(), 2, 1); err == nil {
		t.Fatal("submit accepted an inverted interval")
	}
}

func TestReceiptUsesAuthoritySignature(t *testing.T) {
	h := newHarness(t)
	h.appendN(5)
	rec, err := h.submitter().Submit(context.Background(), 1, 5)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if rec.TSA != h.srv.URL {
		t.Fatalf("receipt names authority %q, want %q", rec.TSA, h.srv.URL)
	}
	if _, err := time.Parse(time.RFC3339, rec.GenTime); err != nil {
		t.Fatalf("genTime is not RFC3339: %v", err)
	}
}
