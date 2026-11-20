package render

import "testing"

func TestPolicyFor(t *testing.T) {
	tests := []struct {
		name string
		want Name
	}{
		{"plain", Plain},
		{"color", Color},
		{"width", Width},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := PolicyFor(tc.name, 20)
			if err != nil {
				t.Fatalf("PolicyFor(%q) returned %v", tc.name, err)
			}
			if p.Name != tc.want {
				t.Errorf("PolicyFor(%q).Name = %q, want %q", tc.name, p.Name, tc.want)
			}
			if p.MaxWidth != 20 {
				t.Errorf("PolicyFor(%q).MaxWidth = %d, want 20", tc.name, p.MaxWidth)
			}
		})
	}
}

func TestApplyPlain(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"text", "connection refused", "connection refused"},
		{"unicode", "café 漢字", "café 漢字"},
		{"bell", "a\ab", "a\\x07b"},
		{"del", "a\x7fb", "a\\x7fb"},
	}
	p, err := PolicyFor("plain", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Apply(p, []byte(tc.in)); got != tc.want {
				t.Errorf("Apply(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestApplyColor(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"red", "\x1b[31mred\x1b[0m", "\x1b[31mred\x1b[0m"},
		{"bold", "\x1b[1mbold\x1b[22m", "\x1b[1mbold\x1b[22m"},
		{"plain text", "no colour here", "no colour here"},
	}
	p, err := PolicyFor("color", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Apply(p, []byte(tc.in)); got != tc.want {
				t.Errorf("Apply(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestApplyWidth(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"fits", "abc", "abc"},
		{"cut", "abcdefgh", "abcd"},
		{"empty", "", ""},
	}
	p, err := PolicyFor("width", 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Apply(p, []byte(tc.in)); got != tc.want {
				t.Errorf("Apply(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
