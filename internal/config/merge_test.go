package config

import (
	"testing"
)

func TestMergeAddsOverlayOnlyKeys(t *testing.T) {
	base := map[string]any{
		"db.host": "db.internal",
		"retries": "3",
	}
	overlay := map[string]any{
		"log.level": "debug",
		"timeout":   "30s",
	}

	got := Merge(base, overlay)

	if v, _ := got["log.level"].(string); v != "debug" {
		t.Fatalf("log.level = %v", got["log.level"])
	}
	if v, _ := got["timeout"].(string); v != "30s" {
		t.Fatalf("timeout = %v", got["timeout"])
	}
	if v, _ := got["db.host"].(string); v != "db.internal" {
		t.Fatalf("db.host = %v", got["db.host"])
	}
	if v, _ := got["retries"].(string); v != "3" {
		t.Fatalf("retries = %v", got["retries"])
	}
}

func TestMergeDoesNotMutateInputs(t *testing.T) {
	base := map[string]any{"a": "1"}
	overlay := map[string]any{"b": "2"}
	Merge(base, overlay)

	if len(base) != 1 || len(overlay) != 1 {
		t.Fatalf("inputs mutated: base=%v overlay=%v", base, overlay)
	}
}

func TestDumpRendersLists(t *testing.T) {
	cfg := newConfig("default")
	cfg.put("features", []string{"metrics", "tracing"}, meta{})
	cfg.put("service.name", "checkout", meta{})

	dump := cfg.Dump()
	if got := dump["features"]; got != "[metrics, tracing]" {
		t.Fatalf("features dump = %q", got)
	}
	if got := dump["service.name"]; got != "checkout" {
		t.Fatalf("service.name dump = %q", got)
	}
}

func TestConfigKeysAreSorted(t *testing.T) {
	cfg := newConfig("default")
	cfg.put("b", "1", meta{})
	cfg.put("a", "2", meta{})
	keys := cfg.Keys()
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
		t.Fatalf("keys = %v", keys)
	}
}
