package loader

import (
	"path/filepath"
	"testing"
	"time"
)

const fixtureRoot = "../../testdata"

func fixture(name string) string { return filepath.Join(fixtureRoot, name) }

func TestLoadPairReadsGoodFixture(t *testing.T) {
	t.Parallel()

	fixedNow := time.Date(2030, 6, 1, 12, 0, 0, 0, time.UTC)
	pair, err := LoadPair(Config{
		CertFile:           fixture("good/leaf.pem"),
		KeyFile:            fixture("good/leaf-key.pem"),
		ExpectedIdentities: []string{"service.internal"},
		Now:                func() time.Time { return fixedNow },
	})
	if err != nil {
		t.Fatalf("LoadPair: %v", err)
	}
	if pair.Leaf == nil {
		t.Fatal("LoadPair returned a pair without a parsed leaf")
	}
	if got := pair.Leaf.Subject.CommonName; got != "service.internal" {
		t.Fatalf("leaf common name = %q, want service.internal", got)
	}
	if !pair.LoadedAt.Equal(fixedNow) {
		t.Fatalf("LoadedAt = %s, want %s", pair.LoadedAt, fixedNow)
	}
	if len(pair.Chain) != 0 {
		t.Fatalf("chain length = %d, want 0", len(pair.Chain))
	}
	if len(pair.CertPEM) == 0 || len(pair.KeyPEM) == 0 {
		t.Fatal("raw PEM material was not retained")
	}
}

func TestLoadPairPreservesPEM(t *testing.T) {
	t.Parallel()

	pair, err := LoadPair(Config{
		CertFile:           fixture("good/leaf.pem"),
		KeyFile:            fixture("good/leaf-key.pem"),
		ExpectedIdentities: []string{"localhost"},
		Now:                func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("LoadPair: %v", err)
	}
	if types := PEMBlockTypes(pair.CertPEM); len(types) != 1 || types[0] != "CERTIFICATE" {
		t.Fatalf("certificate block types = %v, want [CERTIFICATE]", types)
	}
	if types := PEMBlockTypes(pair.KeyPEM); len(types) != 1 || types[0] != "PRIVATE KEY" {
		t.Fatalf("key block types = %v, want [PRIVATE KEY]", types)
	}
}

func TestLoadPairRequiresPaths(t *testing.T) {
	t.Parallel()

	if _, err := LoadPair(Config{KeyFile: "k.pem"}); err == nil {
		t.Fatal("expected an error when the certificate file is missing from the config")
	}
	if _, err := LoadPair(Config{CertFile: "c.pem"}); err == nil {
		t.Fatal("expected an error when the key file is missing from the config")
	}
}

func TestLoadPairMissingFile(t *testing.T) {
	t.Parallel()

	_, err := LoadPair(Config{
		CertFile: fixture("good/does-not-exist.pem"),
		KeyFile:  fixture("good/leaf-key.pem"),
	})
	if err == nil {
		t.Fatal("expected an error for a missing certificate file")
	}
}
