package keyring

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDeriveKEKIsVersionSpecific(t *testing.T) {
	master := bytes.Repeat([]byte{0x42}, MinMasterSecretSize)

	v1, err := DeriveKEK(master, 1)
	if err != nil {
		t.Fatalf("DeriveKEK(1): %v", err)
	}
	v2, err := DeriveKEK(master, 2)
	if err != nil {
		t.Fatalf("DeriveKEK(2): %v", err)
	}

	if bytes.Equal(v1, v2) {
		t.Fatal("different versions derived the same KEK")
	}
	if len(v1) != KeySize || len(v2) != KeySize {
		t.Fatalf("derived key sizes are %d and %d, want %d", len(v1), len(v2), KeySize)
	}
}

func TestParseRejectsInvalidKEKLength(t *testing.T) {
	shortKEK := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x01}, 20))
	doc := []byte(fmt.Sprintf(`{
		"format": 1,
		"keys": [
			{"version": 1, "status": "active", "kek": %q}
		]
	}`, shortKEK))

	if _, err := Parse(doc); err == nil {
		t.Fatal("Parse accepted a 20-byte KEK")
	}
}

func TestLoadRejectsInvalidKEKLength(t *testing.T) {
	shortKEK := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x01}, 16))
	doc := []byte(fmt.Sprintf(`{
		"format": 1,
		"keys": [
			{"version": 1, "status": "active", "kek": %q}
		]
	}`, shortKEK))

	path := filepath.Join(t.TempDir(), "keyring.json")
	if err := os.WriteFile(path, doc, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load accepted a 16-byte KEK")
	}
}

func TestSelectForWriteIgnoresRetiredKeys(t *testing.T) {
	kr := New([]Key{
		testKey(1, StatusRetired, 0x01),
		testKey(2, StatusActive, 0x02),
	})

	key, err := kr.SelectForWrite()
	if err != nil {
		t.Fatalf("SelectForWrite: %v", err)
	}
	if key.Version != 2 {
		t.Fatalf("SelectForWrite returned version %d, want 2", key.Version)
	}
}

func TestSelectForWriteUsesHighestActiveVersion(t *testing.T) {
	kr := New([]Key{
		testKey(1, StatusActive, 0x01),
		testKey(3, StatusActive, 0x03),
		testKey(2, StatusActive, 0x02),
		testKey(4, StatusRetired, 0x04),
	})

	key, err := kr.SelectForWrite()
	if err != nil {
		t.Fatalf("SelectForWrite: %v", err)
	}
	if key.Version != 3 {
		t.Fatalf("SelectForWrite returned version %d, want 3", key.Version)
	}
}

func TestSelectForWriteRequiresActiveKey(t *testing.T) {
	kr := New([]Key{
		testKey(1, StatusRetired, 0x01),
		testKey(2, StatusRetired, 0x02),
	})

	if _, err := kr.SelectForWrite(); err == nil {
		t.Fatal("SelectForWrite accepted a keyring with no active key")
	}
}

func TestRotateMovesWriteSelectionToNewVersion(t *testing.T) {
	kr := New([]Key{testKey(1, StatusActive, 0x01)})
	if err := kr.Rotate(testKey(2, StatusActive, 0x02)); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	key, err := kr.SelectForWrite()
	if err != nil {
		t.Fatalf("SelectForWrite: %v", err)
	}
	if key.Version != 2 {
		t.Fatalf("SelectForWrite returned version %d after rotation, want 2", key.Version)
	}
}
