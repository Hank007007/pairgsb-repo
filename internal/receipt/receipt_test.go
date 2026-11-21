package receipt

import (
	"crypto/ed25519"
	"crypto/rand"
	"path/filepath"
	"testing"
	"time"

	"beaconproof/internal/tsa"
)

func signedReceipt(t *testing.T, priv ed25519.PrivateKey, root string, from, to int64) Receipt {
	t.Helper()
	genTime := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC).Format(time.RFC3339)
	return Receipt{
		FromSeq: from,
		ToSeq:   to,
		Root:    root,
		GenTime: genTime,
		Sig:     tsa.SignMessage(priv, root, genTime),
		TSA:     "test",
	}
}

func TestVerifyReceiptAcceptsSignedReceipt(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	r := signedReceipt(t, priv, "abc123", 1, 10)

	ctx := CheckContext{PublicKey: pub}
	if findings := VerifyReceipt(1, r, ctx); len(findings) != 0 {
		t.Fatalf("valid receipt was rejected: %v", findings)
	}
}

func TestVerifyReceiptRejectsBadSignature(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	_, otherPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	r := signedReceipt(t, otherPriv, "abc123", 1, 10)

	ctx := CheckContext{PublicKey: pub}
	findings := VerifyReceipt(1, r, ctx)
	if len(findings) == 0 {
		t.Fatal("receipt signed by another key was accepted")
	}
	if findings[0].Field != "sig" {
		t.Fatalf("finding names field %q, want sig", findings[0].Field)
	}
}

func TestDirRoundTrip(t *testing.T) {
	dir, err := OpenDir(filepath.Join(t.TempDir(), "receipts"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	r := Receipt{FromSeq: 1, ToSeq: 4, Root: "aa", GenTime: "2026-11-21T10:00:00Z", Sig: "sig", TSA: "test"}
	if _, err := dir.Save(r); err != nil {
		t.Fatalf("save: %v", err)
	}
	all, err := dir.LoadAll()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(all) != 1 || all[0].Root != "aa" || all[0].ToSeq != 4 {
		t.Fatalf("round trip mismatch: %+v", all)
	}
}
