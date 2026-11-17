package keyring

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func testKey(version uint32, status Status, fill byte) Key {
	return Key{
		Version:  version,
		Status:   status,
		Material: bytes.Repeat([]byte{fill}, KeySize),
	}
}

func TestParseOrdersVersions(t *testing.T) {
	doc := []byte(`{
		"format": 1,
		"keys": [
			{"version": 3, "status": "active", "kek": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},
			{"version": 1, "status": "retired", "kek": "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE="},
			{"version": 2, "status": "retired", "kek": "AgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgI="}
		]
	}`)

	kr, err := Parse(doc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := kr.Len(); got != 3 {
		t.Fatalf("Len = %d, want 3", got)
	}

	keys := kr.Keys()
	for i, want := range []uint32{1, 2, 3} {
		if keys[i].Version != want {
			t.Errorf("keys[%d].Version = %d, want %d", i, keys[i].Version, want)
		}
	}
}

func TestParseRejectsEmptyDocument(t *testing.T) {
	if _, err := Parse([]byte(`{"format":1,"keys":[]}`)); err == nil {
		t.Fatal("Parse accepted an empty keyring")
	}
}

func TestParseRejectsDuplicateVersion(t *testing.T) {
	doc := []byte(`{
		"format": 1,
		"keys": [
			{"version": 1, "status": "active", "kek": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},
			{"version": 1, "status": "retired", "kek": "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE="}
		]
	}`)
	if _, err := Parse(doc); err == nil {
		t.Fatal("Parse accepted duplicate versions")
	}
}

func TestParseRejectsUnknownStatus(t *testing.T) {
	doc := []byte(`{
		"format": 1,
		"keys": [
			{"version": 1, "status": "pending", "kek": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}
		]
	}`)
	if _, err := Parse(doc); err == nil {
		t.Fatal("Parse accepted an unknown status")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keyring.json")

	original := New([]Key{
		testKey(1, StatusRetired, 0x01),
		testKey(2, StatusActive, 0x02),
	})
	if err := Save(path, original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("Save wrote an empty document")
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Len() != original.Len() {
		t.Fatalf("loaded %d keys, want %d", loaded.Len(), original.Len())
	}

	got := loaded.Keys()
	want := original.Keys()
	for i := range want {
		if got[i].Version != want[i].Version {
			t.Errorf("keys[%d].Version = %d, want %d", i, got[i].Version, want[i].Version)
		}
		if got[i].Status != want[i].Status {
			t.Errorf("keys[%d].Status = %q, want %q", i, got[i].Status, want[i].Status)
		}
		if !bytes.Equal(got[i].Material, want[i].Material) {
			t.Errorf("keys[%d] material changed across save and load", i)
		}
	}
}

func TestSelectForReadFindsVersion(t *testing.T) {
	kr := New([]Key{
		testKey(1, StatusRetired, 0x01),
		testKey(2, StatusActive, 0x02),
	})

	key, err := kr.SelectForRead(1)
	if err != nil {
		t.Fatalf("SelectForRead(1): %v", err)
	}
	if key.Version != 1 {
		t.Fatalf("Version = %d, want 1", key.Version)
	}

	if _, err := kr.SelectForRead(99); err == nil {
		t.Fatal("SelectForRead accepted an unknown version")
	}
}

func TestRotateAddsVersionAndRetiresPrevious(t *testing.T) {
	kr := New([]Key{testKey(1, StatusActive, 0x01)})

	if err := kr.Rotate(testKey(2, StatusActive, 0x02)); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	if got := kr.Len(); got != 2 {
		t.Fatalf("Len = %d, want 2", got)
	}
	if got := kr.NextVersion(); got != 3 {
		t.Fatalf("NextVersion = %d, want 3", got)
	}

	keys := kr.Keys()
	if keys[0].Version != 1 || keys[0].Status != StatusRetired {
		t.Errorf("version 1 = %+v, want retired", keys[0])
	}
	if keys[1].Version != 2 || keys[1].Status != StatusActive {
		t.Errorf("version 2 = %+v, want active", keys[1])
	}

	old, err := kr.SelectForRead(1)
	if err != nil {
		t.Fatalf("SelectForRead after rotate: %v", err)
	}
	if !bytes.Equal(old.Material, bytes.Repeat([]byte{0x01}, KeySize)) {
		t.Error("retired version lost its key material")
	}
}

func TestRotateRejectsDuplicateVersion(t *testing.T) {
	kr := New([]Key{testKey(1, StatusActive, 0x01)})
	if err := kr.Rotate(testKey(1, StatusActive, 0x02)); err == nil {
		t.Fatal("Rotate accepted a duplicate version")
	}
}

func TestKeysReturnsCopy(t *testing.T) {
	kr := New([]Key{testKey(1, StatusActive, 0x01)})

	keys := kr.Keys()
	keys[0].Material[0] = 0xff

	again := kr.Keys()
	if again[0].Material[0] != 0x01 {
		t.Fatal("Keys exposed the internal key material buffer")
	}
}
