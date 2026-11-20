package scan

import "testing"

func TestWidth(t *testing.T) {
	tests := []struct {
		name string
		r    rune
		want int
	}{
		{"ascii", 'a', 1},
		{"space", ' ', 1},
		{"combining acute", '́', 0},
		{"combining enclosing", '⃝', 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Width(tc.r); got != tc.want {
				t.Errorf("Width(%U) = %d, want %d", tc.r, got, tc.want)
			}
		})
	}
}

func TestCut(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{"fits", "abc", 5, "abc"},
		{"exact", "abc", 3, "abc"},
		{"truncate", "abcdef", 3, "abc"},
		{"zero limit", "abc", 0, ""},
		{"empty", "", 4, ""},
		{"combining kept with base", "éx", 1, "é"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Cut(tc.in, tc.limit); got != tc.want {
				t.Errorf("Cut(%q, %d) = %q, want %q", tc.in, tc.limit, got, tc.want)
			}
		})
	}
}
