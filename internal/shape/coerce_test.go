package shape

import (
	"errors"
	"reflect"
	"testing"
)

func TestCoerceInt(t *testing.T) {
	cases := []struct {
		raw  string
		want int32
	}{
		{"0", 0},
		{"42", 42},
		{"-7", -7},
		{"+13", 13},
		{"2147483647", 2147483647},
		{"-2147483648", -2147483648},
	}
	for _, tc := range cases {
		got, err := CoerceInt(tc.raw)
		if err != nil {
			t.Fatalf("CoerceInt(%q): %v", tc.raw, err)
		}
		if got != tc.want {
			t.Fatalf("CoerceInt(%q) = %d, want %d", tc.raw, got, tc.want)
		}
	}
}

func TestCoerceIntRejectsNonNumeric(t *testing.T) {
	cases := []string{"", "abc", "1.5", "1e3", "0x10", "1 000", " 1"}
	for _, raw := range cases {
		if _, err := CoerceInt(raw); !errors.Is(err, ErrInt) {
			t.Fatalf("CoerceInt(%q) err = %v, want ErrInt", raw, err)
		}
	}
}

func TestCoerceBool(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"true", true},
		{"false", false},
	}
	for _, tc := range cases {
		got, err := CoerceBool(tc.raw)
		if err != nil {
			t.Fatalf("CoerceBool(%q): %v", tc.raw, err)
		}
		if got != tc.want {
			t.Fatalf("CoerceBool(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestCoerceBoolRejectsOtherLiterals(t *testing.T) {
	for _, raw := range []string{"", "maybe", "truee"} {
		if _, err := CoerceBool(raw); !errors.Is(err, ErrBool) {
			t.Fatalf("CoerceBool(%q) err = %v, want ErrBool", raw, err)
		}
	}
}

func TestCoerceStringAppliesClass(t *testing.T) {
	field := Field{Kind: KindString, MaxBytes: 16, Class: ClassSafe}
	got, err := CoerceString("build-42_a.b", field)
	if err != nil {
		t.Fatalf("CoerceString: %v", err)
	}
	if got != "build-42_a.b" {
		t.Fatalf("CoerceString = %q", got)
	}
}

func TestCoerceStringTextAllowsCJK(t *testing.T) {
	field := Field{Kind: KindString, MaxBytes: 24, Class: ClassText}
	got, err := CoerceString("中文昵称", field)
	if err != nil {
		t.Fatalf("CoerceString: %v", err)
	}
	if got != "中文昵称" {
		t.Fatalf("CoerceString = %q", got)
	}
}

func TestCoerceStringEnum(t *testing.T) {
	field := Field{Kind: KindString, MaxBytes: 8, Class: ClassSafe, Enum: []string{"alpha", "beta"}}
	got, err := CoerceString("beta", field)
	if err != nil {
		t.Fatalf("CoerceString: %v", err)
	}
	if got != "beta" {
		t.Fatalf("CoerceString = %q", got)
	}
	if _, err := CoerceString("gamma", field); !errors.Is(err, ErrEnum) {
		t.Fatalf("CoerceString(gamma) err = %v, want ErrEnum", err)
	}
}

func TestCheckClassAnyLeavesTextAlone(t *testing.T) {
	got, err := CheckClass("anything at all", ClassAny)
	if err != nil {
		t.Fatalf("CheckClass: %v", err)
	}
	if got != "anything at all" {
		t.Fatalf("CheckClass = %q", got)
	}
}

func TestLimitBytesUnbounded(t *testing.T) {
	long := "这是一个没有上限的字段"
	got, err := LimitBytes(long, 0)
	if err != nil {
		t.Fatalf("LimitBytes: %v", err)
	}
	if got != long {
		t.Fatalf("LimitBytes = %q", got)
	}
}

func TestLimitBytesWithinBound(t *testing.T) {
	got, err := LimitBytes("short", 16)
	if err != nil {
		t.Fatalf("LimitBytes: %v", err)
	}
	if got != "short" {
		t.Fatalf("LimitBytes = %q", got)
	}
}

func TestFieldNamesStableCopy(t *testing.T) {
	sh := &Shape{
		Name: "t",
		Fields: map[string]Field{
			"z": {Kind: KindString},
			"a": {Kind: KindString},
			"m": {Kind: KindString},
		},
	}
	got := sh.FieldNames()
	want := []string{"a", "m", "z"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FieldNames = %v, want %v", got, want)
	}
}
