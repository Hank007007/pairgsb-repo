package fields_test

import (
	"errors"
	"testing"

	"logline-audit-encode/internal/fields"
)

func TestNormalizeKeepsPlainText(t *testing.T) {
	const in = "invoice read by tenant member"
	if got := fields.Normalize(in); got != in {
		t.Fatalf("Normalize(%q) = %q", in, got)
	}
}

func TestNormalizeReplacesInvalidUTF8(t *testing.T) {
	got := fields.Normalize("a\xffb")
	if got != "a�b" {
		t.Fatalf("Normalize returned %q, want %q", got, "a�b")
	}
}

func TestNormalizeLeavesValueAtLimitAlone(t *testing.T) {
	in := make([]byte, fields.MaxValueBytes)
	for i := range in {
		in[i] = 'a'
	}
	if got := fields.Normalize(string(in)); got != string(in) {
		t.Fatalf("value of exactly %d bytes was changed", fields.MaxValueBytes)
	}
}

func TestSetPreservesOrder(t *testing.T) {
	set := fields.NewSet()
	if err := set.Set("service", "billing-api", fields.SourceCore); err != nil {
		t.Fatalf("Set(service): %v", err)
	}
	if err := set.Set("region", "eu-west-1", fields.SourceExtension); err != nil {
		t.Fatalf("Set(region): %v", err)
	}

	got := set.Fields()
	if len(got) != 2 {
		t.Fatalf("Fields() returned %d fields, want 2", len(got))
	}
	if got[0].Key != "service" || got[0].Value != "billing-api" || got[0].Source != fields.SourceCore {
		t.Errorf("field 0 = %+v", got[0])
	}
	if got[1].Key != "region" || got[1].Value != "eu-west-1" || got[1].Source != fields.SourceExtension {
		t.Errorf("field 1 = %+v", got[1])
	}
}

func TestSetFieldsReturnsCopy(t *testing.T) {
	set := fields.NewSet()
	if err := set.Set("service", "billing-api", fields.SourceCore); err != nil {
		t.Fatalf("Set: %v", err)
	}

	fields1 := set.Fields()
	fields1[0].Value = "mutated"

	if got := set.Fields()[0].Value; got != "billing-api" {
		t.Fatalf("Fields() exposed internal state: got %q", got)
	}
}

func TestErrorsAreDistinct(t *testing.T) {
	if errors.Is(fields.ErrDuplicateField, fields.ErrReservedField) {
		t.Fatal("error values are not distinct")
	}
	if errors.Is(fields.ErrInvalidKey, fields.ErrReservedField) {
		t.Fatal("error values are not distinct")
	}
}
