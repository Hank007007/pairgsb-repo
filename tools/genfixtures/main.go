// Command genfixtures regenerates the key rings and pre-sealed records under
// testdata/. The keys are fixed (not random) so that the fixtures are stable
// across runs; they are test keys only and must never be used for real data.
//
// Run from the repository root:
//
//	go run ./tools/genfixtures
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"kekseal/internal/keystore"
)

const ringV1V2 = `{
  "grace_period": "24h",
  "versions": [
    {
      "version": 1,
      "kek": "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=",
      "created_at": "2025-01-01T00:00:00Z"
    },
    {
      "version": 2,
      "kek": "ICEiIyQlJicoKSorLC0uLzAxMjM0NTY3ODk6Ozw9Pj8=",
      "created_at": "2025-03-01T00:00:00Z"
    }
  ]
}
`

const ringV1Only = `{
  "grace_period": "24h",
  "versions": [
    {
      "version": 1,
      "kek": "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=",
      "created_at": "2025-01-01T00:00:00Z"
    }
  ]
}
`

func main() {
	if err := run("testdata"); err != nil {
		fmt.Fprintln(os.Stderr, "genfixtures:", err)
		os.Exit(1)
	}
}

func run(root string) error {
	if err := os.MkdirAll(filepath.Join(root, "rings"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, "records"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "rings", "ring-v1-v2.json"), []byte(ringV1V2), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "rings", "ring-v1-only.json"), []byte(ringV1Only), 0o644); err != nil {
		return err
	}

	oldRing, err := keystore.ParseRing([]byte(ringV1Only))
	if err != nil {
		return err
	}
	newRing, err := keystore.ParseRing([]byte(ringV1V2))
	if err != nil {
		return err
	}
	oldSealer := keystore.NewSealer(oldRing)
	newSealer := keystore.NewSealer(newRing)

	fixtures := []struct {
		file   string
		tenant string
		id     string
		text   string
		sealer *keystore.Sealer
	}{
		{"tenant-a-0001.json", "acme", "doc-0001", "acme:alpha", oldSealer},
		{"tenant-a-0002.json", "acme", "doc-0002", "acme:beta", oldSealer},
		{"tenant-b-0003.json", "globex", "doc-0003", "globex:gamma", oldSealer},
		{"doc-0004.json", "acme", "doc-0004", "acme:delta", newSealer},
	}
	for _, f := range fixtures {
		rec, err := f.sealer.Seal(f.tenant, f.id, []byte(f.text))
		if err != nil {
			return err
		}
		if err := keystore.WriteRecord(filepath.Join(root, "records", f.file), rec); err != nil {
			return err
		}
	}
	fmt.Printf("wrote %d rings and %d records under %s\n", 2, len(fixtures), root)
	return nil
}
