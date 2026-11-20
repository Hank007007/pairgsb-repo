package limits

import "testing"

func TestBudgetByteAccounting(t *testing.T) {
	b := New()
	if err := b.Bytes(64); err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if err := b.Bytes(36); err != nil {
		t.Fatalf("Bytes: %v", err)
	}

	_, _, _, bytes := b.Used()
	if got, want := bytes, 100; got != want {
		t.Fatalf("bytes = %d, want %d", got, want)
	}
}

func TestBudgetReset(t *testing.T) {
	b := New()
	_ = b.Bytes(5)
	_ = b.Steps(1)
	_ = b.Name(3)
	_ = b.Record()
	b.Reset()

	steps, names, records, bytes := b.Used()
	if steps != 0 || names != 0 || records != 0 || bytes != 0 {
		t.Fatalf("after Reset: %d,%d,%d,%d; want all zero", steps, names, records, bytes)
	}
}
