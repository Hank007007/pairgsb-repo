package manifest

import (
	"bytes"
	"crypto/ed25519"
	"os"
	"testing"

	"cborveil/internal/cbor"
)

func TestParseExampleManifest(t *testing.T) {
	m, err := Parse(readFixture(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Version != "2.4.1" {
		t.Fatalf("version = %q", m.Version)
	}
	if m.Size != 4096 {
		t.Fatalf("size = %d", m.Size)
	}
	if m.KeyID != "vendor-2024" {
		t.Fatalf("key_id = %q", m.KeyID)
	}
	if len(m.Sig) != 64 {
		t.Fatalf("sig = %d bytes", len(m.Sig))
	}
	if len(m.Targets) != 1 {
		t.Fatalf("targets = %d", len(m.Targets))
	}
	tg := m.Targets[0]
	if tg.Model != "P1" {
		t.Fatalf("model = %q", tg.Model)
	}
	if tg.URL != "https://ota.example.com/p1/2.4.1.bin" {
		t.Fatalf("url = %q", tg.URL)
	}
	if tg.Size != 2048 || tg.Offset != 0 {
		t.Fatalf("size/offset = %d/%d", tg.Size, tg.Offset)
	}
	if len(tg.SHA256) != 32 {
		t.Fatalf("sha256 = %d bytes", len(tg.SHA256))
	}
}

func TestSelectSingleTarget(t *testing.T) {
	m, err := Parse(readFixture(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tg, err := Select(m, "P1")
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if tg.Model != "P1" {
		t.Fatalf("selected %q", tg.Model)
	}
}

func TestSelectUnknownModel(t *testing.T) {
	m, err := Parse(readFixture(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := Select(m, "X9"); err == nil {
		t.Fatal("expected error for unknown model")
	}
}

func TestSigningBytesStable(t *testing.T) {
	m, err := Parse(readFixture(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	a, err := SigningBytes(m)
	if err != nil {
		t.Fatalf("SigningBytes: %v", err)
	}
	b, err := SigningBytes(m)
	if err != nil {
		t.Fatalf("SigningBytes: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("SigningBytes is not stable for the same manifest")
	}
	if len(a) == 0 {
		t.Fatal("SigningBytes returned no bytes")
	}
}

// TestVerifyAcceptsSignedManifest builds a manifest, signs whatever
// SigningBytes reports for it, and checks that Verify accepts the result.
func TestVerifyAcceptsSignedManifest(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	target := cbor.NewMap()
	target.Set("url", "https://ota.example.com/p1/1.0.0.bin")
	target.Set("size", uint64(512))
	target.Set("model", "P1")
	target.Set("offset", uint64(0))
	target.Set("sha256", bytes.Repeat([]byte{0x11}, 32))

	root := cbor.NewMap()
	root.Set("sig", make([]byte, ed25519.SignatureSize))
	root.Set("size", uint64(512))
	root.Set("key_id", "test-key")
	root.Set("targets", []any{target})
	root.Set("version", "1.0.0")

	raw, err := cbor.Encode(root)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	m, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	msg, err := SigningBytes(m)
	if err != nil {
		t.Fatalf("SigningBytes: %v", err)
	}
	root.Set("sig", ed25519.Sign(priv, msg))

	raw, err = cbor.Encode(root)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	m, err = Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := Verify(m, pub); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func readFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/example.cbor")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	return data
}
