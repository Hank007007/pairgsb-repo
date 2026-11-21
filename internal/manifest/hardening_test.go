package manifest

import (
	"bytes"
	"errors"
	"math"
	"testing"

	"cborveil/internal/cbor"
)

func validTarget(model string, offset, size uint64) *cbor.Map {
	t := cbor.NewMap()
	t.Set("model", model)
	t.Set("url", "https://ota.example.com/"+model+".bin")
	t.Set("sha256", bytes.Repeat([]byte{0x11}, 32))
	t.Set("size", size)
	t.Set("offset", offset)
	return t
}

func validManifest(t *testing.T, size uint64, targets []any) *cbor.Map {
	t.Helper()
	root := cbor.NewMap()
	root.Set("version", "2.4.1")
	root.Set("size", size)
	root.Set("targets", targets)
	root.Set("sig", bytes.Repeat([]byte{0xa5}, 64))
	root.Set("key_id", "vendor-2024")
	return root
}

func encode(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := cbor.Encode(v)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return raw
}

func TestParseRejectsInvalidTargets(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*cbor.Map)
	}{
		{"url empty", func(m *cbor.Map) { m.Set("url", "") }},
		{"sha256 short", func(m *cbor.Map) { m.Set("sha256", bytes.Repeat([]byte{0x11}, 16)) }},
		{"size zero", func(m *cbor.Map) { m.Set("size", uint64(0)) }},
		{"size negative", func(m *cbor.Map) { m.Set("size", int64(-1)) }},
		{"size over int64", func(m *cbor.Map) { m.Set("size", uint64(math.MaxInt64)+1) }},
		{"offset negative", func(m *cbor.Map) { m.Set("offset", int64(-1)) }},
		{"range exceeds package", func(m *cbor.Map) {
			m.Set("offset", uint64(800))
			m.Set("size", uint64(800))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := validTarget("P1", 0, 100)
			tc.mutate(target)
			raw := encode(t, validManifest(t, 1000, []any{target}))
			if _, err := Parse(raw); err == nil {
				t.Fatal("expected Parse to reject target")
			}
		})
	}
}

func TestParseRejectsDuplicateModels(t *testing.T) {
	raw := encode(t, validManifest(t, 1000, []any{
		validTarget("P1", 0, 100),
		validTarget("P1", 100, 100),
	}))
	if _, err := Parse(raw); !errors.Is(err, ErrManifestInvalid) {
		t.Fatalf("Parse error = %v, want ErrManifestInvalid", err)
	}
}

func TestParseIgnoresTargetUnknownKeys(t *testing.T) {
	target := validTarget("P1", 0, 100)
	target.Set("version", "9.9.9")
	m, err := Parse(encode(t, validManifest(t, 1000, []any{target})))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Version != "2.4.1" {
		t.Fatalf("version = %q, want top-level 2.4.1", m.Version)
	}
}

func TestParseRejectsDuplicateTopLevelKeys(t *testing.T) {
	// {"version": "1.0.0", "version": "9.9.9"}
	raw := []byte{
		0xa2,
		0x67, 'v', 'e', 'r', 's', 'i', 'o', 'n', 0x65, '1', '.', '0', '.', '0',
		0x67, 'v', 'e', 'r', 's', 'i', 'o', 'n', 0x65, '9', '.', '9', '.', '9',
	}
	if _, err := Parse(raw); !errors.Is(err, ErrNotCanonical) {
		t.Fatalf("Parse error = %v, want ErrNotCanonical", err)
	}
}

func TestSigningBytesExcludesSignatureFields(t *testing.T) {
	makeManifest := func(keyID string) *Manifest {
		t.Helper()
		root := validManifest(t, 1000, []any{validTarget("P1", 0, 100)})
		root.Set("key_id", keyID)
		m, err := Parse(encode(t, root))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		return m
	}

	a, err := SigningBytes(makeManifest("vendor-2024"))
	if err != nil {
		t.Fatalf("SigningBytes: %v", err)
	}
	b, err := SigningBytes(makeManifest("vendor-2025"))
	if err != nil {
		t.Fatalf("SigningBytes: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("signing bytes depend on key_id:\n%x\n%x", a, b)
	}
	for _, leak := range [][]byte{[]byte("vendor-2024"), []byte("vendor-2025")} {
		if bytes.Contains(a, leak) {
			t.Fatalf("signing bytes contain %q", leak)
		}
	}
}

func TestSigningBytesIndependentOfMapInsertionOrder(t *testing.T) {
	build := func(reverse bool) *Manifest {
		t.Helper()
		target := cbor.NewMap()
		keys := []string{"model", "url", "sha256", "size", "offset"}
		if reverse {
			keys = []string{"offset", "size", "sha256", "url", "model"}
		}
		vals := map[string]any{
			"model":  "P1",
			"url":    "https://ota.example.com/P1.bin",
			"sha256": bytes.Repeat([]byte{0x11}, 32),
			"size":   uint64(100),
			"offset": uint64(0),
		}
		for _, k := range keys {
			target.Set(k, vals[k])
		}
		m, err := Parse(encode(t, validManifest(t, 1000, []any{target})))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		return m
	}

	a, err := SigningBytes(build(false))
	if err != nil {
		t.Fatalf("SigningBytes: %v", err)
	}
	b, err := SigningBytes(build(true))
	if err != nil {
		t.Fatalf("SigningBytes: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("signing bytes depend on insertion order:\n%x\n%x", a, b)
	}
}

func TestSelectRequiresExactModel(t *testing.T) {
	raw := encode(t, validManifest(t, 1000, []any{
		validTarget("P10", 0, 100),
		validTarget("P1-Pro", 100, 100),
		validTarget("P1", 200, 100),
	}))
	m, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	target, err := Select(m, "P1")
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if target.Model != "P1" {
		t.Fatalf("selected %q, want P1", target.Model)
	}
	if _, err := Select(m, "P"); !errors.Is(err, ErrNoMatchingTarget) {
		t.Fatalf("Select(P) error = %v, want ErrNoMatchingTarget", err)
	}
}

func TestSelectRejectsAmbiguousMatches(t *testing.T) {
	m := &Manifest{Targets: []Target{{Model: "P1"}, {Model: "P1"}}}
	if _, err := Select(m, "P1"); !errors.Is(err, ErrManifestInvalid) {
		t.Fatalf("Select error = %v, want ErrManifestInvalid", err)
	}
}
