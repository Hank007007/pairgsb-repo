package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunDumpsManifest(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"../../internal/manifest/testdata/example.cbor"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	want := []string{
		"version: 2.4.1",
		"size: 4096",
		"key_id: vendor-2024",
		"sig: 64 bytes",
		"targets: 1",
		"target[0]: model=P1 url=https://ota.example.com/p1/2.4.1.bin sha256=" +
			"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f" +
			" size=2048 offset=0",
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != len(want) {
		t.Fatalf("got %d lines:\n%s", len(lines), stdout.String())
	}
	for i, w := range want {
		if lines[i] != w {
			t.Fatalf("line %d = %q, want %q", i, lines[i], w)
		}
	}
}

func TestRunSelectsModel(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-model", "P1", "../../internal/manifest/testdata/example.cbor"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "selected: model=P1 url=https://ota.example.com/p1/2.4.1.bin") {
		t.Fatalf("unexpected output:\n%s", stdout.String())
	}
}

func TestRunMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"does-not-exist.cbor"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "otadump:") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}
