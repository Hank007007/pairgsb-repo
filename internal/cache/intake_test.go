package cache

import (
	"testing"

	"ptrloop/internal/wire"
)

func TestClampTreatsTTLAsUnsigned(t *testing.T) {
	tests := []struct {
		ttl     uint32
		want    int32
		clamped bool
	}{
		{0, 0, false},
		{300, 300, false},
		{uint32(MaxTTL), MaxTTL, false},
		{uint32(MaxTTL) + 1, MaxTTL, true},
		{0x80000000, MaxTTL, true},
		{0xFFFFFFFF, MaxTTL, true},
	}
	for _, tc := range tests {
		got := Clamp(tc.ttl)
		if got.Seconds != tc.want || got.Clamped != tc.clamped {
			t.Errorf("Clamp(%#x) = %+v, want %d clamped=%v", tc.ttl, got, tc.want, tc.clamped)
		}
	}
}

func TestInBailiwickUsesLabelBoundaries(t *testing.T) {
	tests := []struct {
		owner string
		zone  string
		want  bool
	}{
		{"example.com", "example.com", true},
		{"www.example.com", "example.com", true},
		{"evil-example.com", "example.com", false},
		{"ample.com", "example.com", false},
		{"example.com.evil.net", "example.com", false},
	}
	for _, tc := range tests {
		if got := inBailiwick(tc.owner, tc.zone); got != tc.want {
			t.Errorf("inBailiwick(%q, %q) = %v, want %v", tc.owner, tc.zone, got, tc.want)
		}
	}
}

func TestNormalizeNameExpandsDecimalEscapes(t *testing.T) {
	name := wire.Name{Labels: []string{`foo\065bar`, "Example", "COM"}}
	got, ok := normalizeName(name)
	if !ok {
		t.Fatal("normalizeName rejected a valid escaped name")
	}
	if want := "fooabar.example.com"; got != want {
		t.Fatalf("normalizeName = %q, want %q", got, want)
	}
}

func TestNormalizeNameRejectsBadEscapes(t *testing.T) {
	for _, label := range []string{`foo\x41`, `foo\999`, "bad_label", ""} {
		if _, ok := normalizeLabel(label); ok {
			t.Errorf("normalizeLabel(%q) succeeded, want failure", label)
		}
	}
}

func TestScreenCaptureExpectations(t *testing.T) {
	tests := []struct {
		name     string
		accepted int
	}{
		{"normal-a.bin", 1},
		{"case-mismatch.bin", 0},
		{"cname-case-hop.bin", 2},
		{"cname-chain-in.bin", 2},
		{"cname-chain-out.bin", 0},
		{"rdata-name-pointer.bin", 1},
		{"suffix-confusion.bin", 0},
		{"ttl-huge.bin", 1},
		{"ttl-wrap.bin", 1},
		{"ttl-zero.bin", 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := parseCapture(t, tc.name)
			d := Screen(msg, msg.Questions[0].Name)
			if got := len(d.Cacheable); got != tc.accepted {
				t.Fatalf("accepted = %d, want %d (rejected=%v)", got, tc.accepted, d.Rejected)
			}
		})
	}
}

func TestScreenClampsLargeTTL(t *testing.T) {
	for _, name := range []string{"ttl-huge.bin", "ttl-wrap.bin"} {
		msg := parseCapture(t, name)
		d := Screen(msg, msg.Questions[0].Name)
		if len(d.Cacheable) != 1 {
			t.Fatalf("%s: accepted = %d, want 1", name, len(d.Cacheable))
		}
		ttl := d.Cacheable[0].TTL
		if ttl.Seconds != MaxTTL || !ttl.Clamped {
			t.Fatalf("%s: ttl = %+v, want clamped %d", name, ttl, MaxTTL)
		}
	}
}

func TestScreenRejectsCNAMECycle(t *testing.T) {
	a := wire.Name{Labels: []string{"a", "example", "com"}}
	b := wire.Name{Labels: []string{"b", "example", "com"}}
	msg := &wire.Message{Answers: []wire.Record{
		{Name: a, Type: wire.TypeCNAME, Data: wire.RData{Names: []wire.Name{b}}},
		{Name: b, Type: wire.TypeCNAME, Data: wire.RData{Names: []wire.Name{a}}},
	}}
	d := Screen(msg, a)
	if len(d.Cacheable) != 0 {
		t.Fatalf("accepted = %d, want 0", len(d.Cacheable))
	}
}

func TestScreenRejectsUnrelatedAnswer(t *testing.T) {
	qname := wire.Name{Labels: []string{"www", "example", "com"}}
	other := wire.Name{Labels: []string{"other", "example", "com"}}
	msg := &wire.Message{Answers: []wire.Record{
		{Name: qname, Type: wire.TypeA, TTL: 60},
		{Name: other, Type: wire.TypeA, TTL: 60},
	}}
	d := Screen(msg, qname)
	if len(d.Cacheable) != 1 {
		t.Fatalf("accepted = %d, want 1", len(d.Cacheable))
	}
	if len(d.Rejected) != 1 || d.Rejected[0] != "other.example.com." {
		t.Fatalf("rejected = %v, want [other.example.com.]", d.Rejected)
	}
}
