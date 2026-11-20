package scan

import "testing"

func TestNext(t *testing.T) {
	tests := []struct {
		name string
		in   string
		ok   bool
		raw  string
		kind Kind
	}{
		{"reset", "\x1b[0mrest", true, "\x1b[0m", KindCSI},
		{"colour", "\x1b[31mred", true, "\x1b[31m", KindCSI},
		{"long parameter", "\x1b[123m", true, "\x1b[123m", KindCSI},
		{"plain text", "hello", false, "", 0},
		{"lone escape", "\x1b", false, "", 0},
		{"open bracket", "\x1b[", false, "", 0},
		{"not a sequence prefix", "x\x1b[31m", false, "", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			seq, ok := Next([]byte(tc.in))
			if ok != tc.ok {
				t.Fatalf("Next(%q) ok = %v, want %v", tc.in, ok, tc.ok)
			}
			if !ok {
				return
			}
			if string(seq.Raw) != tc.raw {
				t.Errorf("Next(%q) raw = %q, want %q", tc.in, seq.Raw, tc.raw)
			}
			if seq.Kind != tc.kind {
				t.Errorf("Next(%q) kind = %v, want %v", tc.in, seq.Kind, tc.kind)
			}
		})
	}
}

func TestDecodeRune(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		r     rune
		size  int
		class Class
	}{
		{"ascii", "a", 'a', 1, Text},
		{"latin", "é", 'é', 2, Text},
		{"cjk", "漢", '漢', 3, Text},
		{"bell", "\a", '\a', 1, C0},
		{"nul", "\x00", 0, 1, C0},
		{"format", "​", '​', 3, ZeroWidth},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := DecodeRune([]byte(tc.in))
			if d.Rune != tc.r || d.Size != tc.size || d.Class != tc.class {
				t.Errorf("DecodeRune(%q) = %+v, want rune %q size %d class %v",
					tc.in, d, tc.r, tc.size, tc.class)
			}
		})
	}
}

func TestVisible(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"colour", "\x1b[31m", "\\x1b[31m"},
		{"reset", "\x1b[0m", "\\x1b[0m"},
		{"bell in payload", "\x1b]0;a\ab", "\\x1b]0;a\\x07b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Visible([]byte(tc.in)); got != tc.want {
				t.Errorf("Visible(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestToken(t *testing.T) {
	tests := []struct {
		r    rune
		want string
	}{
		{0x07, "\\x07"},
		{0x1b, "\\x1b"},
		{0x7f, "\\x7f"},
		{0x9b, "\\u009b"},
		{0x202e, "\\u202e"},
	}
	for _, tc := range tests {
		if got := Token(tc.r); got != tc.want {
			t.Errorf("Token(%U) = %q, want %q", tc.r, got, tc.want)
		}
	}
}
