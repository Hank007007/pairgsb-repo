package keystore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testTenant    = "acme"
	testPlaintext = "quarterly revenue forecast: 4.2M"
)

func mustParseRing(t *testing.T, body string) *Ring {
	t.Helper()
	ring, err := ParseRing([]byte(body))
	if err != nil {
		t.Fatalf("ParseRing: %v", err)
	}
	return ring
}

func testRing(t *testing.T) *Ring {
	t.Helper()
	return mustParseRing(t, `{
  "grace_period": "24h",
  "versions": [
    {"version": 1, "kek": "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=", "created_at": "2025-01-01T00:00:00Z"}
  ]
}`)
}

func TestSealOpenRoundTrip(t *testing.T) {
	ring := testRing(t)
	sealer := NewSealer(ring)

	rec, err := sealer.Seal(testTenant, "rec-0001", []byte(testPlaintext))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if rec.Alg != AlgAESGCM {
		t.Fatalf("Alg = %q, want %q", rec.Alg, AlgAESGCM)
	}
	if rec.KeyVersion != 1 {
		t.Fatalf("KeyVersion = %d, want 1", rec.KeyVersion)
	}
	if strings.Contains(string(rec.Ciphertext), testPlaintext) {
		t.Fatal("ciphertext contains plaintext")
	}

	got, err := sealer.Open(rec)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if string(got) != testPlaintext {
		t.Fatalf("Open = %q, want %q", got, testPlaintext)
	}
}

func TestRecordJSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rec.json")

	rec, err := NewSealer(testRing(t)).Seal(testTenant, "rec-0002", []byte("hello"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if err := WriteRecord(path, rec); err != nil {
		t.Fatalf("WriteRecord: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("record is not valid JSON: %v", err)
	}
	for _, field := range []string{"id", "tenant", "key_version", "alg", "nonce", "wrapped_dek", "ciphertext"} {
		if _, ok := fields[field]; !ok {
			t.Errorf("record JSON missing field %q", field)
		}
	}

	back, err := ReadRecord(path)
	if err != nil {
		t.Fatalf("ReadRecord: %v", err)
	}
	plaintext, err := NewSealer(testRing(t)).Open(back)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if string(plaintext) != "hello" {
		t.Fatalf("Open = %q, want %q", plaintext, "hello")
	}
}

func TestSealRejectsEmptyIdentifiers(t *testing.T) {
	sealer := NewSealer(testRing(t))
	if _, err := sealer.Seal("", "rec", []byte("x")); err == nil {
		t.Fatal("Seal with empty tenant succeeded")
	}
	if _, err := sealer.Seal("acme", "", []byte("x")); err == nil {
		t.Fatal("Seal with empty id succeeded")
	}
}

func TestSealUsesNewestVersion(t *testing.T) {
	ring := mustParseRing(t, `{
  "versions": [
    {"version": 1, "kek": "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="},
    {"version": 2, "kek": "ICEiIyQlJicoKSorLC0uLzAxMjM0NTY3ODk6Ozw9Pj8="}
  ]
}`)
	rec, err := NewSealer(ring).Seal("acme", "rec-0003", []byte("v2"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if rec.KeyVersion != 2 {
		t.Fatalf("KeyVersion = %d, want 2", rec.KeyVersion)
	}
}

func TestBuildAADIsDeterministic(t *testing.T) {
	rec := &Record{ID: "rec-1", Tenant: "acme", KeyVersion: 1}
	first := BuildAAD(rec)
	second := BuildAAD(&Record{ID: "rec-1", Tenant: "acme", KeyVersion: 1})
	if string(first) != string(second) {
		t.Fatal("BuildAAD is not deterministic for identical records")
	}
}

func TestOpenFixtureRecords(t *testing.T) {
	ring, err := LoadRing(filepath.Join("..", "..", "testdata", "rings", "ring-v1-v2.json"))
	if err != nil {
		t.Fatalf("LoadRing: %v", err)
	}
	sealer := NewSealer(ring)

	cases := []struct {
		file string
		want string
	}{
		{"tenant-a-0001.json", "acme:alpha"},
		{"tenant-a-0002.json", "acme:beta"},
		{"tenant-b-0003.json", "globex:gamma"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			rec, err := ReadRecord(filepath.Join("..", "..", "testdata", "records", tc.file))
			if err != nil {
				t.Fatalf("ReadRecord: %v", err)
			}
			plaintext, err := sealer.Open(rec)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			if string(plaintext) != tc.want {
				t.Fatalf("Open = %q, want %q", plaintext, tc.want)
			}
		})
	}
}

func TestOpenUnknownVersion(t *testing.T) {
	twoVersions := mustParseRing(t, `{
  "versions": [
    {"version": 1, "kek": "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="},
    {"version": 2, "kek": "ICEiIyQlJicoKSorLC0uLzAxMjM0NTY3ODk6Ozw9Pj8="}
  ]
}`)
	rec, err := NewSealer(twoVersions).Seal("acme", "rec-0004", []byte("x"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if _, err := NewSealer(testRing(t)).Open(rec); !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("Open err = %v, want ErrUnknownVersion", err)
	}
}

func TestLoadRingMetadata(t *testing.T) {
	ring, err := LoadRing(filepath.Join("..", "..", "testdata", "rings", "ring-v1-v2.json"))
	if err != nil {
		t.Fatalf("LoadRing: %v", err)
	}
	if len(ring.Versions) != 2 {
		t.Fatalf("len(Versions) = %d, want 2", len(ring.Versions))
	}
	if ring.GracePeriod != 24*time.Hour {
		t.Fatalf("GracePeriod = %v, want 24h", ring.GracePeriod)
	}
	version, kek, err := ring.ActiveKEK()
	if err != nil {
		t.Fatalf("ActiveKEK: %v", err)
	}
	if version != 2 {
		t.Fatalf("ActiveKEK version = %d, want 2", version)
	}
	if len(kek) != 32 {
		t.Fatalf("ActiveKEK length = %d, want 32", len(kek))
	}
	if _, err := ring.KEK(1); err != nil {
		t.Fatalf("KEK(1): %v", err)
	}
}

func TestParseRingRejectsMalformedInput(t *testing.T) {
	cases := map[string]string{
		"bad json":     `{"versions": [`,
		"no versions":  `{"versions": []}`,
		"zero version": `{"versions": [{"version": 0, "kek": "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="}]}`,
		"short kek":    `{"versions": [{"version": 1, "kek": "AAEC"}]}`,
		"bad grace":    `{"grace_period": "soon", "versions": [{"version": 1, "kek": "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseRing([]byte(body)); err == nil {
				t.Fatal("ParseRing succeeded, want error")
			}
		})
	}
}

func TestRingMarshalRoundTrip(t *testing.T) {
	ring := testRing(t)
	data, err := ring.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	back, err := ParseRing(data)
	if err != nil {
		t.Fatalf("ParseRing: %v", err)
	}
	if back.GracePeriod != ring.GracePeriod {
		t.Fatalf("GracePeriod = %v, want %v", back.GracePeriod, ring.GracePeriod)
	}
	if len(back.Versions) != len(ring.Versions) {
		t.Fatalf("len(Versions) = %d, want %d", len(back.Versions), len(ring.Versions))
	}
	if string(back.Versions[0].KEK) != string(ring.Versions[0].KEK) {
		t.Fatal("KEK changed across marshal round trip")
	}
}

func TestRotateMovesAllRecords(t *testing.T) {
	oldRing := testRing(t)
	rec1, err := NewSealer(oldRing).Seal("acme", "rec-r1", []byte("one"))
	if err != nil {
		t.Fatal(err)
	}
	rec2, err := NewSealer(oldRing).Seal("acme", "rec-r2", []byte("two"))
	if err != nil {
		t.Fatal(err)
	}

	newRing := mustParseRing(t, `{
  "versions": [
    {"version": 1, "kek": "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="},
    {"version": 2, "kek": "ICEiIyQlJicoKSorLC0uLzAxMjM0NTY3ODk6Ozw9Pj8="}
  ]
}`)
	records := []*Record{rec1, rec2}
	res, err := NewSealer(newRing).RotateKEK(records)
	if err != nil {
		t.Fatalf("RotateKEK: %v", err)
	}
	if res.TargetVersion != 2 {
		t.Fatalf("TargetVersion = %d, want 2", res.TargetVersion)
	}
	if res.Rotated != 2 || res.Skipped != 0 {
		t.Fatalf("Rotation = %+v, want Rotated=2 Skipped=0", res)
	}

	sealer := NewSealer(newRing)
	for i, rec := range records {
		if rec.KeyVersion != 2 {
			t.Fatalf("record %d KeyVersion = %d, want 2", i, rec.KeyVersion)
		}
		plaintext, err := sealer.Open(rec)
		if err != nil {
			t.Fatalf("Open record %d: %v", i, err)
		}
		want := []string{"one", "two"}[i]
		if string(plaintext) != want {
			t.Fatalf("record %d = %q, want %q", i, plaintext, want)
		}
	}
}

func TestRotateSkipsRecordsAlreadyOnTarget(t *testing.T) {
	ring := mustParseRing(t, `{
  "versions": [
    {"version": 1, "kek": "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="},
    {"version": 2, "kek": "ICEiIyQlJicoKSorLC0uLzAxMjM0NTY3ODk6Ozw9Pj8="}
  ]
}`)
	rec, err := NewSealer(ring).Seal("acme", "rec-r3", []byte("already new"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := NewSealer(ring).RotateKEK([]*Record{rec})
	if err != nil {
		t.Fatalf("RotateKEK: %v", err)
	}
	if res.Rotated != 0 || res.Skipped != 1 {
		t.Fatalf("Rotation = %+v, want Rotated=0 Skipped=1", res)
	}
	if _, err := NewSealer(ring).Open(rec); err != nil {
		t.Fatalf("Open: %v", err)
	}
}
