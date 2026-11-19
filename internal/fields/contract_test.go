package fields_test

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"logline-audit-encode/internal/fields"
)

func TestNormalizeTruncatesOnRuneBoundary(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "drops_partial_euro",
			in:   strings.Repeat("a", 1022) + "€€",
			want: strings.Repeat("a", 1022),
		},
		{
			name: "drops_whole_euro",
			in:   strings.Repeat("a", 1023) + "€",
			want: strings.Repeat("a", 1023),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fields.Normalize(tc.in)
			if got != tc.want {
				t.Fatalf("Normalize produced %d bytes, want %d", len(got), len(tc.want))
			}
			if !utf8.ValidString(got) {
				t.Fatalf("Normalize produced invalid UTF-8: %q", got)
			}
			if len(got) > fields.MaxValueBytes {
				t.Fatalf("Normalize produced %d bytes, limit is %d", len(got), fields.MaxValueBytes)
			}
		})
	}
}

func TestNormalizeKeepsValueAtLimitWithMultibyteRune(t *testing.T) {
	in := strings.Repeat("a", 1021) + "€"
	if got := fields.Normalize(in); got != in {
		t.Fatalf("Normalize altered a value of exactly %d bytes", fields.MaxValueBytes)
	}
}

func TestSetRejectsInvalidKeys(t *testing.T) {
	keys := []string{
		"",
		"1abc",
		"-abc",
		".abc",
		"a b",
		"x y=z",
		"a=b",
		`a"b`,
		`a\b`,
		"a\nb",
		"a/b",
		"é",
	}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			set := fields.NewSet()
			err := set.Set(key, "value", fields.SourceExtension)
			if !errors.Is(err, fields.ErrInvalidKey) {
				t.Fatalf("Set(%q) error = %v, want ErrInvalidKey", key, err)
			}
			if set.Len() != 0 {
				t.Fatalf("Set(%q) added %d fields after rejection", key, set.Len())
			}
		})
	}
}

func TestSetRejectsReservedExtensionNames(t *testing.T) {
	names := []string{
		"timestamp",
		"service",
		"event_id",
		"level",
		"tenant_id",
		"trace_id",
		"actor",
		"action",
		"resource",
		"path",
		"user_agent",
		"message",
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			set := fields.NewSet()
			err := set.Set(name, "override", fields.SourceExtension)
			if !errors.Is(err, fields.ErrReservedField) {
				t.Fatalf("Set(%q) error = %v, want ErrReservedField", name, err)
			}
			if set.Len() != 0 {
				t.Fatalf("Set(%q) added %d fields after rejection", name, set.Len())
			}
		})
	}
}

func TestSetAllowsReservedCoreNames(t *testing.T) {
	set := fields.NewSet()
	if err := set.Set("level", "info", fields.SourceCore); err != nil {
		t.Fatalf("Set(level, SourceCore): %v", err)
	}
}

func TestSetRejectsDuplicateNamesWithoutOverwriting(t *testing.T) {
	set := fields.NewSet()
	if err := set.Set("region", "eu-west-1", fields.SourceExtension); err != nil {
		t.Fatalf("first Set: %v", err)
	}

	err := set.Set("region", "us-east-1", fields.SourceExtension)
	if !errors.Is(err, fields.ErrDuplicateField) {
		t.Fatalf("second Set error = %v, want ErrDuplicateField", err)
	}

	got := set.Fields()
	if len(got) != 1 || got[0].Value != "eu-west-1" {
		t.Fatalf("duplicate Set changed the field set: %+v", got)
	}
}
