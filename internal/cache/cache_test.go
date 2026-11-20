package cache

import (
	"os"
	"path/filepath"
	"testing"

	"ptrloop/internal/limits"
	"ptrloop/internal/wire"
)

func readCapture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return raw
}

func parseCapture(t *testing.T, name string) *wire.Message {
	t.Helper()
	msg, err := wire.Parse(readCapture(t, name), limits.New())
	if err != nil {
		t.Fatalf("Parse %s: %v", name, err)
	}
	return msg
}

func TestScreenNormalAnswer(t *testing.T) {
	msg := parseCapture(t, "normal-a.bin")

	d := Screen(msg, msg.Questions[0].Name)
	if got, want := len(d.Cacheable), 1; got != want {
		t.Fatalf("cacheable = %d, want %d", got, want)
	}
	e := d.Cacheable[0]
	if got, want := e.Owner, "www.example.com."; got != want {
		t.Fatalf("owner = %q, want %q", got, want)
	}
	if got, want := e.Type, wire.TypeA; got != want {
		t.Fatalf("type = %d, want %d", got, want)
	}
	if got, want := e.TTL.Seconds, int32(300); got != want {
		t.Fatalf("ttl = %d, want %d", got, want)
	}
	if e.TTL.Clamped {
		t.Fatal("ttl was marked clamped although it is below the ceiling")
	}
}

func TestScreenNXDomainHasNothingToCache(t *testing.T) {
	msg := parseCapture(t, "nxdomain.bin")

	d := Screen(msg, msg.Questions[0].Name)
	if len(d.Cacheable) != 0 {
		t.Fatalf("cacheable = %d, want 0", len(d.Cacheable))
	}
}

func TestScreenEmptyMessage(t *testing.T) {
	d := Screen(nil, wire.Name{Labels: []string{"example", "com"}})
	if len(d.Cacheable) != 0 || len(d.Rejected) != 0 {
		t.Fatalf("empty message produced %d cacheable / %d rejected",
			len(d.Cacheable), len(d.Rejected))
	}
}

func TestInBailiwickEqualAndBelow(t *testing.T) {
	if !inBailiwick("example.com", "example.com") {
		t.Fatal("zone should be in its own bailiwick")
	}
	if !inBailiwick("www.example.com", "example.com") {
		t.Fatal("child should be in the zone bailiwick")
	}
}

func TestClampBelowCeiling(t *testing.T) {
	got := Clamp(300)
	if got.Seconds != 300 || got.Clamped {
		t.Fatalf("Clamp(300) = %+v, want 300 unclamped", got)
	}
}

func TestClampAboveCeiling(t *testing.T) {
	got := Clamp(9 * 24 * 60 * 60)
	if got.Seconds != MaxTTL || !got.Clamped {
		t.Fatalf("Clamp(9d) = %+v, want %d clamped", got, MaxTTL)
	}
}

func TestClampZero(t *testing.T) {
	got := Clamp(0)
	if got.Seconds != 0 || got.Clamped {
		t.Fatalf("Clamp(0) = %+v, want 0 unclamped", got)
	}
}
