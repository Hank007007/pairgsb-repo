package limits

import (
	"errors"
	"testing"
)

func TestBudgetCeilings(t *testing.T) {
	b := New()
	if err := b.Steps(MaxDecompressionSteps); err != nil {
		t.Fatalf("Steps(%d): %v", MaxDecompressionSteps, err)
	}
	if err := b.Steps(1); !errors.Is(err, ErrExhausted) {
		t.Fatalf("Steps over ceiling = %v, want ErrExhausted", err)
	}

	b = New()
	if err := b.Name(MaxNameLength); err != nil {
		t.Fatalf("Name(%d): %v", MaxNameLength, err)
	}
	if err := b.Name(MaxNameLength + 1); !errors.Is(err, ErrExhausted) {
		t.Fatalf("Name over ceiling = %v, want ErrExhausted", err)
	}

	b = New()
	for i := 0; i < MaxRecords; i++ {
		if err := b.Record(); err != nil {
			t.Fatalf("Record %d: %v", i+1, err)
		}
	}
	if err := b.Record(); !errors.Is(err, ErrExhausted) {
		t.Fatalf("Record over ceiling = %v, want ErrExhausted", err)
	}
}
