package keystore

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func twoVersionTestRing(t *testing.T) *Ring {
	t.Helper()
	return mustParseRing(t, `{
  "versions": [
    {"version": 1, "kek": "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="},
    {"version": 2, "kek": "ICEiIyQlJicoKSorLC0uLzAxMjM0NTY3ODk6Ozw9Pj8="}
  ]
}`)
}

func cloneRecord(rec *Record) *Record {
	out := *rec
	out.Nonce = append([]byte(nil), rec.Nonce...)
	out.WrappedDEK = append([]byte(nil), rec.WrappedDEK...)
	out.Ciphertext = append([]byte(nil), rec.Ciphertext...)
	return &out
}

func TestOpenRejectsTamperedHeaderFields(t *testing.T) {
	ring := testRing(t)
	rec, err := NewSealer(ring).Seal("acme", "doc-0001", []byte("top secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*Record)
	}{
		{"tenant", func(r *Record) { r.Tenant = "globex" }},
		{"id", func(r *Record) { r.ID = "doc-9999" }},
		{"key_version", func(r *Record) { r.KeyVersion = 2 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tampered := cloneRecord(rec)
			tc.mutate(tampered)

			plaintext, err := NewSealer(ring).Open(tampered)
			if !errors.Is(err, ErrAuthentication) {
				t.Fatalf("Open err = %v, want ErrAuthentication", err)
			}
			if plaintext != nil {
				t.Fatalf("Open returned plaintext %q alongside the error", plaintext)
			}
		})
	}
}

func TestBuildAADEncodesFieldBoundaries(t *testing.T) {
	first := BuildAAD(&Record{ID: "ab", Tenant: "c", KeyVersion: 1})
	second := BuildAAD(&Record{ID: "a", Tenant: "bc", KeyVersion: 1})
	if string(first) == string(second) {
		t.Fatal("distinct id/tenant pairs produced identical AAD")
	}

	oldVersion := BuildAAD(&Record{ID: "a", Tenant: "b", KeyVersion: 1})
	newVersion := BuildAAD(&Record{ID: "a", Tenant: "b", KeyVersion: 2})
	if string(oldVersion) == string(newVersion) {
		t.Fatal("distinct key versions produced identical AAD")
	}
}

func TestOpenRejectsUnsupportedAlgorithm(t *testing.T) {
	ring := testRing(t)
	rec, err := NewSealer(ring).Seal("acme", "doc-0001", []byte("top secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	for _, alg := range []string{"plain", "", "AES-GCM", "aes-gcm ", "future"} {
		t.Run(fmt.Sprintf("alg=%q", alg), func(t *testing.T) {
			tampered := cloneRecord(rec)
			tampered.Alg = alg
			tampered.Ciphertext = []byte("must not leak")

			plaintext, err := NewSealer(ring).Open(tampered)
			if !errors.Is(err, ErrUnsupportedAlgorithm) {
				t.Fatalf("Open err = %v, want ErrUnsupportedAlgorithm", err)
			}
			if plaintext != nil {
				t.Fatalf("Open returned plaintext %q alongside the error", plaintext)
			}
		})
	}
}

func TestParseRingRejectsDuplicateVersions(t *testing.T) {
	body := `{
  "versions": [
    {"version": 1, "kek": "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="},
    {"version": 1, "kek": "ICEiIyQlJicoKSorLC0uLzAxMjM0NTY3ODk6Ozw9Pj8="}
  ]
}`
	ring, err := ParseRing([]byte(body))
	if !errors.Is(err, ErrDuplicateVersion) {
		t.Fatalf("ParseRing err = %v, want ErrDuplicateVersion", err)
	}
	if ring != nil {
		t.Fatal("ParseRing returned a ring for duplicate versions")
	}
}

func TestOpenRejectsExpiredRetiredVersion(t *testing.T) {
	ring := mustParseRing(t, `{
  "grace_period": "24h",
  "versions": [
    {"version": 1, "kek": "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=", "retired_at": "2020-01-01T00:00:00Z"}
  ]
}`)
	rec, err := ReadRecord(filepath.Join("..", "..", "testdata", "records", "tenant-a-0001.json"))
	if err != nil {
		t.Fatalf("ReadRecord: %v", err)
	}

	plaintext, err := NewSealer(ring).Open(rec)
	if !errors.Is(err, ErrVersionRetired) {
		t.Fatalf("Open err = %v, want ErrVersionRetired", err)
	}
	if plaintext != nil {
		t.Fatalf("Open returned plaintext %q alongside the error", plaintext)
	}
}

func TestOpenAllowsRetiredVersionInsideGracePeriod(t *testing.T) {
	retiredAt := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	ring := mustParseRing(t, fmt.Sprintf(`{
  "grace_period": "24h",
  "versions": [
    {"version": 1, "kek": "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=", "retired_at": %q}
  ]
}`, retiredAt))

	rec, err := NewSealer(testRing(t)).Seal("acme", "doc-0001", []byte("still valid"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	plaintext, err := NewSealer(ring).Open(rec)
	if err != nil {
		t.Fatalf("Open inside grace period: %v", err)
	}
	if string(plaintext) != "still valid" {
		t.Fatalf("Open = %q, want %q", plaintext, "still valid")
	}
}

func TestRotateIsAtomicWhenAnyRecordFails(t *testing.T) {
	oldRing := testRing(t)
	good, err := NewSealer(oldRing).Seal("acme", "good", []byte("good"))
	if err != nil {
		t.Fatalf("Seal good: %v", err)
	}

	unrelatedRing := mustParseRing(t, `{
  "versions": [
    {"version": 1, "kek": "ICEiIyQlJicoKSorLC0uLzAxMjM0NTY3ODk6Ozw9Pj8="}
  ]
}`)
	bad, err := NewSealer(unrelatedRing).Seal("acme", "bad", []byte("bad"))
	if err != nil {
		t.Fatalf("Seal bad: %v", err)
	}

	goodBefore := cloneRecord(good)
	badBefore := cloneRecord(bad)
	res, err := NewSealer(twoVersionTestRing(t)).RotateKEK([]*Record{good, bad})
	if !errors.Is(err, ErrRotationFailed) {
		t.Fatalf("RotateKEK err = %v, want ErrRotationFailed", err)
	}
	if !errors.Is(err, ErrAuthentication) {
		t.Fatalf("RotateKEK err = %v, want underlying ErrAuthentication", err)
	}
	if res.Rotated != 0 || res.Skipped != 0 {
		t.Fatalf("Rotation = %+v, want zero counts on failure", res)
	}
	if !reflect.DeepEqual(good, goodBefore) {
		t.Fatal("good record was modified by a failed rotation")
	}
	if !reflect.DeepEqual(bad, badBefore) {
		t.Fatal("bad record was modified by a failed rotation")
	}

	if _, err := NewSealer(oldRing).Open(good); err != nil {
		t.Fatalf("good record no longer opens with its original ring: %v", err)
	}
}

func TestRotateIsAtomicWhenVersionIsUnknown(t *testing.T) {
	threeVersionRing := &Ring{Versions: []KeyVersion{
		{Version: 1, KEK: fixedKEK(1)},
		{Version: 2, KEK: fixedKEK(2)},
		{Version: 3, KEK: fixedKEK(3)},
	}}
	rec, err := NewSealer(threeVersionRing).Seal("acme", "v3", []byte("v3"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	before := cloneRecord(rec)

	res, err := NewSealer(&Ring{Versions: []KeyVersion{
		{Version: 1, KEK: fixedKEK(1)},
		{Version: 2, KEK: fixedKEK(2)},
	}}).RotateKEK([]*Record{rec})
	if !errors.Is(err, ErrRotationFailed) || !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("RotateKEK err = %v, want ErrRotationFailed wrapping ErrUnknownVersion", err)
	}
	if res.Rotated != 0 || res.Skipped != 0 {
		t.Fatalf("Rotation = %+v, want zero counts on failure", res)
	}
	if !reflect.DeepEqual(rec, before) {
		t.Fatal("record was modified by a failed rotation")
	}
}

func TestRotateRejectsUnsupportedAlgorithmAtomically(t *testing.T) {
	ring := testRing(t)
	rec, err := NewSealer(ring).Seal("acme", "rec", []byte("secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	rec.Alg = "plain"
	before := cloneRecord(rec)

	res, err := NewSealer(twoVersionTestRing(t)).RotateKEK([]*Record{rec})
	if !errors.Is(err, ErrRotationFailed) || !errors.Is(err, ErrUnsupportedAlgorithm) {
		t.Fatalf("RotateKEK err = %v, want ErrRotationFailed wrapping ErrUnsupportedAlgorithm", err)
	}
	if res.Rotated != 0 || res.Skipped != 0 {
		t.Fatalf("Rotation = %+v, want zero counts on failure", res)
	}
	if !reflect.DeepEqual(rec, before) {
		t.Fatal("record was modified by a failed rotation")
	}
}

func TestRotateMigratesLegacyFixtureToAuthenticatedHeader(t *testing.T) {
	rec, err := ReadRecord(filepath.Join("..", "..", "testdata", "records", "tenant-a-0001.json"))
	if err != nil {
		t.Fatalf("ReadRecord: %v", err)
	}
	ring := twoVersionTestRing(t)

	res, err := NewSealer(ring).RotateKEK([]*Record{rec})
	if err != nil {
		t.Fatalf("RotateKEK: %v", err)
	}
	if res.Rotated != 1 || res.Skipped != 0 {
		t.Fatalf("Rotation = %+v, want Rotated=1 Skipped=0", res)
	}
	if rec.KeyVersion != 2 {
		t.Fatalf("KeyVersion = %d, want 2", rec.KeyVersion)
	}
	plaintext, err := NewSealer(ring).Open(rec)
	if err != nil {
		t.Fatalf("Open rotated legacy record: %v", err)
	}
	if string(plaintext) != "acme:alpha" {
		t.Fatalf("Open = %q, want %q", plaintext, "acme:alpha")
	}

	tampered := cloneRecord(rec)
	tampered.Tenant = "globex"
	if _, err := NewSealer(ring).Open(tampered); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("Open tampered rotated record err = %v, want ErrAuthentication", err)
	}
}

func fixedKEK(fill byte) []byte {
	kek := make([]byte, 32)
	for i := range kek {
		kek[i] = fill
	}
	return kek
}
