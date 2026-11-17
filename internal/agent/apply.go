package agent

import (
	"fmt"
	"os"
	"path/filepath"

	"sealrelay/internal/bundle"
)

// SecretsDir is the directory, relative to the agent root, that holds the
// materialized secret files.
const SecretsDir = "secrets"

// ApplyBundle materializes secrets under root/secrets.
func ApplyBundle(root string, secrets []bundle.Secret) error {
	dir := filepath.Join(root, SecretsDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("agent: create secrets dir: %w", err)
	}
	for _, sec := range secrets {
		if err := bundle.ValidateName(sec.Name); err != nil {
			return err
		}
		path := filepath.Join(dir, sec.Name)
		if err := writeFileAtomic(path, sec.Value); err != nil {
			return fmt.Errorf("agent: write %q: %w", sec.Name, err)
		}
	}
	return nil
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".sealrelay-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
