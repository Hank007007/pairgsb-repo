package chain

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T, limit uint64) (string, *Writer) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "keys"), 0o700); err != nil {
		t.Fatal(err)
	}
	kp, err := GenerateKeyPair("k1")
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveKeys(filepath.Join(dir, "keys"), kp); err != nil {
		t.Fatal(err)
	}
	w, err := OpenWriter(dir, limit, kp)
	if err != nil {
		t.Fatal(err)
	}
	return dir, w
}

func TestAppendAndVerifySingleSegment(t *testing.T) {
	dir, w := newTestStore(t, 64)
	for i := 0; i < 5; i++ {
		if _, err := w.Append("alice", "deploy", `{"id":1}`, "ok"); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	res, err := Verify(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK() {
		t.Fatalf("verification failed: %v", res.Findings)
	}
	if res.Segments != 1 {
		t.Fatalf("Segments = %d, want 1", res.Segments)
	}
	if res.Records != 5 {
		t.Fatalf("Records = %d, want 5", res.Records)
	}
	if res.HeadHash == "" {
		t.Fatal("HeadHash is empty")
	}
}

func TestRecordChain(t *testing.T) {
	_, w := newTestStore(t, 64)
	var hashes []string
	for i := 0; i < 4; i++ {
		r, err := w.Append("bob", "policy", "p", "ok")
		if err != nil {
			t.Fatal(err)
		}
		if r.Seq != uint64(i+1) {
			t.Fatalf("seq = %d, want %d", r.Seq, i+1)
		}
		if i == 0 {
			if r.Prev != ZeroHash {
				t.Fatalf("first prev = %s, want zero hash", r.Prev)
			}
		} else if r.Prev != hashes[len(hashes)-1] {
			t.Fatalf("record %d prev = %s, want %s", i, r.Prev, hashes[len(hashes)-1])
		}
		hashes = append(hashes, r.Hash)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSegmentRotation(t *testing.T) {
	dir, w := newTestStore(t, 3)
	for i := 0; i < 7; i++ {
		if _, err := w.Append("carol", "rotate", "x", "ok"); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	nums, err := SegmentNumbers(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(nums) != 3 {
		t.Fatalf("sealed segments = %v, want 3 checkpoint files", nums)
	}

	res, err := Verify(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK() {
		t.Fatalf("verification failed: %v", res.Findings)
	}
}

func TestCheckpointSignature(t *testing.T) {
	dir, w := newTestStore(t, 64)
	if _, err := w.Append("dave", "act", "p", "ok"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	_, ckptPath := SegmentPaths(dir, 1)
	cp, err := ReadCheckpoint(ckptPath)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := LoadPublicKey(filepath.Join(dir, "keys"), cp.KeyID)
	if err != nil {
		t.Fatal(err)
	}
	if !cp.Verify(pub) {
		t.Fatal("checkpoint signature did not verify")
	}
	if cp.Count != 1 || cp.FirstSeq != 1 || cp.LastSeq != 1 {
		t.Fatalf("checkpoint range = [%d,%d] count %d", cp.FirstSeq, cp.LastSeq, cp.Count)
	}
	if len(cp.Hash()) != 64 {
		t.Fatalf("checkpoint hash length = %d", len(cp.Hash()))
	}
}

func TestSealOpenRoundTrip(t *testing.T) {
	dir, w := newTestStore(t, 4)
	for i := 0; i < 4; i++ {
		if _, err := w.Append("erin", "seed", "payload", "ok"); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	seg, err := OpenSegment(dir, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	recs, bad, err := seg.ReadRecords()
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) != 0 {
		t.Fatalf("unreadable lines: %v", bad)
	}
	if len(recs) != 4 {
		t.Fatalf("records = %d, want 4", len(recs))
	}
	root, err := RootOf(recs)
	if err != nil {
		t.Fatal(err)
	}
	cp, err := ReadCheckpoint(seg.Checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if root != cp.Root {
		t.Fatalf("recomputed root %s != checkpoint root %s", root, cp.Root)
	}
}

func TestVerifyEmptyStore(t *testing.T) {
	dir, w := newTestStore(t, 64)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	res, err := Verify(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK() {
		t.Fatalf("empty store reported findings: %v", res.Findings)
	}
}

func TestAppendAfterReopen(t *testing.T) {
	dir, w := newTestStore(t, 64)
	for i := 0; i < 2; i++ {
		if _, err := w.Append("frank", "a", "p", "ok"); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	kp := &KeyPair{}
	prv, err := LoadPrivateKey(filepath.Join(dir, "keys"), "k1")
	if err != nil {
		t.Fatal(err)
	}
	pub, err := LoadPublicKey(filepath.Join(dir, "keys"), "k1")
	if err != nil {
		t.Fatal(err)
	}
	kp.ID, kp.Prv, kp.Pub = "k1", prv, pub

	w2, err := OpenWriter(dir, 64, kp)
	if err != nil {
		t.Fatal(err)
	}
	r, err := w2.Append("frank", "b", "q", "ok")
	if err != nil {
		t.Fatal(err)
	}
	if r.Seq != 3 {
		t.Fatalf("seq after reopen = %d, want 3", r.Seq)
	}
	if err := w2.Close(); err != nil {
		t.Fatal(err)
	}

	res, err := Verify(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK() {
		t.Fatalf("verification failed after reopen: %v", res.Findings)
	}
}
