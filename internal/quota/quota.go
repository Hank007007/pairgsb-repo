// Package quota tracks how much of an extraction has been spent.
package quota

import (
	"fmt"

	"github.com/pairgsb/unpackfence/internal/fault"
)

// MaxEntries is the number of entries a single package may contain.
const MaxEntries = 200000

// Budget holds the limits for a single extraction and the amount spent so far.
type Budget struct {
	MaxBytes int64
	MaxRatio float64

	Used int64
}

// New returns a Budget with the given limits. A limit of zero leaves that
// dimension unbounded.
func New(maxBytes int64, maxRatio float64) *Budget {
	return &Budget{
		MaxBytes: maxBytes,
		MaxRatio: maxRatio,
	}
}

// Add records the size of an entry against the budget.
func (b *Budget) Add(n int64) error {
	if n > 0 {
		b.Used += n
	}
	if b.MaxBytes > 0 && b.Used > b.MaxBytes {
		return fmt.Errorf("%w: %d bytes", fault.ErrBudgetExceeded, b.Used)
	}
	return nil
}

// ObserveRatio compares the bytes written so far with the compressed input
// consumed so far.
func (b *Budget) ObserveRatio(compressed, written int64) error {
	if b.MaxRatio <= 0 || compressed <= 0 || written <= 0 {
		return nil
	}
	ratio := float64(written) / float64(compressed)
	if ratio > b.MaxRatio {
		return fmt.Errorf("%w: %.1f", fault.ErrRatioExceeded, ratio)
	}
	return nil
}
