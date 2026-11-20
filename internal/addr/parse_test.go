package addr

import "testing"

func TestSplitListSingle(t *testing.T) {
	got := SplitList("alice@example.com")
	if len(got) != 1 || got[0] != "alice@example.com" {
		t.Fatalf("SplitList = %#v", got)
	}
}

func TestSplitListTwoPlain(t *testing.T) {
	got := SplitList("alice@example.com, bob@example.com")
	if len(got) != 2 {
		t.Fatalf("SplitList returned %d entries: %#v", len(got), got)
	}
	if got[0] != "alice@example.com" || got[1] != "bob@example.com" {
		t.Fatalf("SplitList = %#v", got)
	}
}

func TestSplitListTrimsSpaces(t *testing.T) {
	got := SplitList("  alice@example.com ,bob@example.com  ")
	if len(got) != 2 || got[0] != "alice@example.com" || got[1] != "bob@example.com" {
		t.Fatalf("SplitList = %#v", got)
	}
}

func TestSplitListEmpty(t *testing.T) {
	if got := SplitList("   "); len(got) != 0 {
		t.Fatalf("SplitList on blank input = %#v", got)
	}
}

func TestParseOneWithAngle(t *testing.T) {
	a := ParseOne("Alice <alice@example.com>")
	if a.Display != "Alice" || a.Local != "alice" || a.Domain != "example.com" {
		t.Fatalf("ParseOne = %#v", a)
	}
}

func TestParseOneBareSpec(t *testing.T) {
	a := ParseOne("alice@example.com")
	if a.Display != "" || a.Local != "alice" || a.Domain != "example.com" {
		t.Fatalf("ParseOne = %#v", a)
	}
}

func TestParseOneSpec(t *testing.T) {
	if got := ParseOne("Alice <alice@example.com>").Spec(); got != "alice@example.com" {
		t.Fatalf("Spec = %q", got)
	}
}

func TestSplitSpec(t *testing.T) {
	local, domain, ok := SplitSpec("alice@example.com")
	if !ok || local != "alice" || domain != "example.com" {
		t.Fatalf("SplitSpec = %q, %q, %v", local, domain, ok)
	}
}

func TestSplitSpecRejectsMalformed(t *testing.T) {
	if _, _, ok := SplitSpec("no-at-sign"); ok {
		t.Fatal("SplitSpec accepted a spec with no @")
	}
}

func TestDisplayNamePlain(t *testing.T) {
	if got := DisplayName("Alice"); got != "Alice" {
		t.Fatalf("DisplayName = %q", got)
	}
}

func TestFormatWithDisplay(t *testing.T) {
	got := Format(Address{Display: "Alice", Local: "alice", Domain: "example.com"})
	if got != "Alice <alice@example.com>" {
		t.Fatalf("Format = %q", got)
	}
}

func TestFormatWithoutDisplay(t *testing.T) {
	got := Format(Address{Local: "alice", Domain: "example.com"})
	if got != "alice@example.com" {
		t.Fatalf("Format = %q", got)
	}
}
