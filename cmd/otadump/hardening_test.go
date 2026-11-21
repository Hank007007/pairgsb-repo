package main

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cborveil/internal/cbor"
)

func targetMap(model string, offset, size uint64) *cbor.Map {
	t := cbor.NewMap()
	t.Set("model", model)
	t.Set("url", "https://ota.example.com/"+model+".bin")
	t.Set("sha256", bytes.Repeat([]byte{0x11}, 32))
	t.Set("size", size)
	t.Set("offset", offset)
	return t
}

func manifestMap(size uint64, targets []any) *cbor.Map {
	m := cbor.NewMap()
	m.Set("version", "2.4.1")
	m.Set("size", size)
	m.Set("targets", targets)
	m.Set("sig", bytes.Repeat([]byte{0xa5}, 64))
	m.Set("key_id", "vendor-2024")
	return m
}

func writeTempManifest(t *testing.T, v any) string {
	t.Helper()
	raw, err := cbor.Encode(v)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	path := filepath.Join(t.TempDir(), "manifest.cbor")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestRunRejectsIncidentTargets(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*cbor.Map)
	}{
		{"size zero", func(m *cbor.Map) { m.Set("size", uint64(0)) }},
		{"size negative", func(m *cbor.Map) { m.Set("size", int64(-1)) }},
		{"size over int64", func(m *cbor.Map) { m.Set("size", uint64(math.MaxInt64)+1) }},
		{"url empty", func(m *cbor.Map) { m.Set("url", "") }},
		{"sha256 short", func(m *cbor.Map) { m.Set("sha256", bytes.Repeat([]byte{0x11}, 16)) }},
		{"range exceeds package", func(m *cbor.Map) {
			m.Set("offset", uint64(800))
			m.Set("size", uint64(800))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := targetMap("P1", 0, 100)
			tc.mutate(target)
			path := writeTempManifest(t, manifestMap(1000, []any{target}))

			var stdout, stderr bytes.Buffer
			if code := run([]string{path}, &stdout, &stderr); code == 0 {
				t.Fatalf("exit 0, stdout:\n%s", stdout.String())
			}
			if strings.Contains(stdout.String(), "target[") {
				t.Fatalf("rejected manifest printed targets:\n%s", stdout.String())
			}
		})
	}
}

func TestRunRejectsDuplicateModels(t *testing.T) {
	path := writeTempManifest(t, manifestMap(1000, []any{
		targetMap("P1", 0, 100),
		targetMap("P1", 100, 100),
	}))
	var stdout, stderr bytes.Buffer
	if code := run([]string{path}, &stdout, &stderr); code == 0 {
		t.Fatalf("exit 0, stdout:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "target[") {
		t.Fatalf("rejected manifest printed targets:\n%s", stdout.String())
	}
}

func TestRunDoesNotLetTargetUnknownKeysOverrideTopLevel(t *testing.T) {
	target := targetMap("P1", 0, 100)
	target.Set("version", "9.9.9")
	path := writeTempManifest(t, manifestMap(1000, []any{target}))

	var stdout, stderr bytes.Buffer
	if code := run([]string{path}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "version: 2.4.1\n") {
		t.Fatalf("top-level version was overwritten:\n%s", stdout.String())
	}
}

func TestRunSelectsExactModel(t *testing.T) {
	path := writeTempManifest(t, manifestMap(1000, []any{
		targetMap("P10", 0, 100),
		targetMap("P1-Pro", 100, 100),
		targetMap("P1", 200, 100),
	}))

	var stdout, stderr bytes.Buffer
	if code := run([]string{"-model", "P1", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "selected: model=P1 ") {
		t.Fatalf("wrong target selected:\n%s", stdout.String())
	}
}
