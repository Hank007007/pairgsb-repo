package policy

import (
	"testing"
	"time"
)

func TestAllowsConfiguredPurpose(t *testing.T) {
	for _, purpose := range []string{"sign", "attest", "timestamp"} {
		p := Policy{Purposes: []string{purpose}}
		if !p.Allows(purpose) {
			t.Fatalf("Allows(%q) = false, want true", purpose)
		}
	}
}

func TestAllowsRejectsEmptyAllowList(t *testing.T) {
	p := Policy{}
	if p.Allows("sign") {
		t.Fatal("Allows(sign) = true with an empty allow-list")
	}
}

func TestPayloadAllowed(t *testing.T) {
	p := Policy{MaxPayloadBytes: 16}

	if !p.PayloadAllowed(16) {
		t.Fatal("PayloadAllowed(16) = false, want true at the limit")
	}
	if p.PayloadAllowed(17) {
		t.Fatal("PayloadAllowed(17) = true, want false above the limit")
	}
}

func TestExpired(t *testing.T) {
	base := time.Date(2030, 3, 1, 12, 0, 0, 0, time.UTC)

	t.Run("no NotAfter", func(t *testing.T) {
		p := Policy{}
		if p.Expired(base.Add(1000 * time.Hour)) {
			t.Fatal("Expired = true for a key without NotAfter")
		}
	})

	t.Run("before NotAfter", func(t *testing.T) {
		p := Policy{NotAfter: base}
		if p.Expired(base.Add(-time.Second)) {
			t.Fatal("Expired = true before NotAfter")
		}
	})

	t.Run("at NotAfter", func(t *testing.T) {
		p := Policy{NotAfter: base}
		if !p.Expired(base) {
			t.Fatal("Expired = false at NotAfter")
		}
	})
}

func TestHourWindowAlignsToNaturalHour(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*3600)
	now := time.Date(2030, 3, 1, 14, 37, 12, 500, loc)

	got := HourWindow(now)
	want := time.Date(2030, 3, 1, 14, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("HourWindow(%v) = %v, want %v", now, got, want)
	}
}

func TestCounterCountsInsideOneHour(t *testing.T) {
	var c Counter
	base := time.Date(2030, 3, 1, 9, 15, 0, 0, time.UTC)

	for i := 1; i <= 3; i++ {
		if got := c.Observe(base.Add(time.Duration(i) * time.Minute)); got != i {
			t.Fatalf("Observe #%d = %d, want %d", i, got, i)
		}
	}
	if got := c.Used(base.Add(3 * time.Minute)); got != 3 {
		t.Fatalf("Used = %d, want 3", got)
	}
}

func TestCounterResetsInNextHour(t *testing.T) {
	var c Counter
	base := time.Date(2030, 3, 1, 9, 50, 0, 0, time.UTC)

	c.Observe(base)
	c.Observe(base.Add(time.Minute))

	next := time.Date(2030, 3, 1, 10, 5, 0, 0, time.UTC)
	if got := c.Used(next); got != 0 {
		t.Fatalf("Used after window rollover = %d, want 0", got)
	}
	if got := c.Observe(next); got != 1 {
		t.Fatalf("Observe after window rollover = %d, want 1", got)
	}
}
