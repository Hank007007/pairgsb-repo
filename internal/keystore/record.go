package keystore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Record is one sealed secret. The DEK that protects Ciphertext is itself
// wrapped by the KEK identified by KeyVersion.
type Record struct {
	ID         string `json:"id"`
	Tenant     string `json:"tenant"`
	KeyVersion int    `json:"key_version"`
	Alg        string `json:"alg"`
	Nonce      []byte `json:"nonce"`
	WrappedDEK []byte `json:"wrapped_dek"`
	Ciphertext []byte `json:"ciphertext"`
}

// ReadRecord decodes a record previously written by WriteRecord.
func ReadRecord(path string) (*Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRecord, err)
	}
	return &rec, nil
}

// WriteRecord atomically replaces path with the JSON encoding of rec.
func WriteRecord(path string, rec *Record) error {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileAtomic(path, data, 0o600)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".kekseal-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
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

func validateRecordShape(rec *Record) error {
	if rec == nil {
		return errors.Join(ErrInvalidRecord, errors.New("nil record"))
	}
	if rec.ID == "" {
		return errors.Join(ErrInvalidRecord, errors.New("missing id"))
	}
	if rec.Tenant == "" {
		return errors.Join(ErrInvalidRecord, errors.New("missing tenant"))
	}
	if rec.KeyVersion <= 0 {
		return errors.Join(ErrInvalidRecord, errors.New("invalid key version"))
	}
	return nil
}
