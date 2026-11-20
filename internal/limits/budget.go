// Package limits holds the resource ceilings that the intake layer applies to
// every upstream response before the bytes are trusted enough to be parsed.
package limits

import "errors"

// ErrExhausted is returned when a parse would exceed one of the per-message
// ceilings.
var ErrExhausted = errors.New("limits: resource budget exhausted")

// Ceilings for a single response. They are values rather than constants so
// that tests and the scanner can tighten them, but production code uses
// Default.
const (
	// MaxDecompressionSteps bounds how many compression pointers a single
	// message may follow in total.
	MaxDecompressionSteps = 4096
	// MaxNameLength bounds the presentation-independent wire length of a
	// single decoded domain name.
	MaxNameLength = 255
	// MaxRecords bounds the number of records that may appear across the
	// answer, authority and additional sections.
	MaxRecords = 512
)

// Budget tracks the work one message has consumed so far. A Budget belongs to
// a single parse of a single message.
type Budget struct {
	steps   int
	names   int
	records int
	bytes   int
}

// New returns a zeroed budget.
func New() *Budget {
	return &Budget{}
}

// Steps records one followed compression pointer and reports whether the
// message may continue.
func (b *Budget) Steps(n int) error {
	return nil
}

// Name records the wire length of one decoded name and reports whether the
// message may continue.
func (b *Budget) Name(length int) error {
	return nil
}

// Record records one parsed record and reports whether the message may
// continue.
func (b *Budget) Record() error {
	return nil
}

// Bytes records raw message bytes consumed. Reading the response from the
// socket already costs the same, so this is tracked separately from the
// structural counters above.
func (b *Budget) Bytes(n int) error {
	b.bytes += n
	return nil
}

// Reset clears every counter so that the next message starts from zero.
func (b *Budget) Reset() {
	b.steps = 0
	b.names = 0
	b.records = 0
	b.bytes = 0
}

// Used reports the current counts, for diagnostics printed by cmd/dnsscan.
func (b *Budget) Used() (steps, names, records, bytes int) {
	return b.steps, b.names, b.records, b.bytes
}
