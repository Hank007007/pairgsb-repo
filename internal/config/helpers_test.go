package config

import (
	"testing"

	"refgate/internal/resolve"
)

func loadOptions(t *testing.T, base, overlay, scope string) Options {
	t.Helper()
	return Options{
		BasePath:    base,
		OverlayPath: overlay,
		Scope:       scope,
		Vars:        map[string]string{"SERVICE_NAME": "checkout", "REGION": "us-east-1"},
		Resolver:    resolverFor(t),
	}
}

func resolverFor(t *testing.T) *resolve.Resolver {
	t.Helper()
	kv, err := resolve.NewKVProvider("../../testdata/kv.json")
	if err != nil {
		t.Fatalf("load kv firmware: %v", err)
	}
	registry := resolve.NewRegistry()
	registry.Register(kv)
	return resolve.NewResolver(registry, resolve.NewCache(0))
}

func mustLoad(t *testing.T, opts Options) *Config {
	t.Helper()
	cfg, err := Load(opts)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}
