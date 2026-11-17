package derive_test

import (
	"bytes"
	"testing"

	"sealrelay/internal/derive"
)

func TestDeriveKeyBindsBundleAndRecipient(t *testing.T) {
	shared := bytes.Repeat([]byte{0x42}, 32)
	recipientA := bytes.Repeat([]byte{0x11}, 32)
	recipientB := bytes.Repeat([]byte{0x22}, 32)

	base, err := derive.DeriveKey(shared, "payments", recipientA)
	if err != nil {
		t.Fatal(err)
	}
	otherBundle, err := derive.DeriveKey(shared, "payroll", recipientA)
	if err != nil {
		t.Fatal(err)
	}
	otherRecipient, err := derive.DeriveKey(shared, "payments", recipientB)
	if err != nil {
		t.Fatal(err)
	}
	again, err := derive.DeriveKey(shared, "payments", recipientA)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(base, otherBundle) {
		t.Fatal("different bundle names produced the same content key")
	}
	if bytes.Equal(base, otherRecipient) {
		t.Fatal("different recipients produced the same content key")
	}
	if !bytes.Equal(base, again) {
		t.Fatal("identical inputs produced different content keys")
	}
}
