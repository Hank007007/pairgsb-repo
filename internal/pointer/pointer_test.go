package pointer

import "testing"

func TestParseRootAndTokens(t *testing.T) {
	cases := []struct {
		ptr    string
		tokens []string
	}{
		{"/title", []string{"title"}},
		{"/metadata/author", []string{"metadata", "author"}},
		{"/tags/0", []string{"tags", "0"}},
	}
	for _, tc := range cases {
		got, err := Parse(tc.ptr)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.ptr, err)
		}
		if len(got) != len(tc.tokens) {
			t.Fatalf("Parse(%q) = %v, want %v", tc.ptr, got, tc.tokens)
		}
		for i := range got {
			if got[i] != tc.tokens[i] {
				t.Fatalf("Parse(%q) = %v, want %v", tc.ptr, got, tc.tokens)
			}
		}
	}
}

func TestParseEmpty(t *testing.T) {
	tokens, err := Parse("")
	if err != nil {
		t.Fatalf("Parse(\"\"): %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("Parse(\"\") = %v, want no tokens", tokens)
	}
}

func TestParseMissingLeadingSlash(t *testing.T) {
	if _, err := Parse("title"); err == nil {
		t.Fatal("expected error for pointer without leading slash")
	}
}

func TestParseEscapes(t *testing.T) {
	cases := []struct {
		ptr    string
		tokens []string
	}{
		{"/a~1b", []string{"a/b"}},
		{"/a~0b", []string{"a~b"}},
		{"/~01", []string{"~1"}},
		{"/", []string{""}},
		{"/metadata/", []string{"metadata", ""}},
	}
	for _, tc := range cases {
		got, err := Parse(tc.ptr)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.ptr, err)
		}
		if len(got) != len(tc.tokens) {
			t.Fatalf("Parse(%q) = %v, want %v", tc.ptr, got, tc.tokens)
		}
		for i := range got {
			if got[i] != tc.tokens[i] {
				t.Fatalf("Parse(%q) = %v, want %v", tc.ptr, got, tc.tokens)
			}
		}
	}
}

func TestParseMalformedEscape(t *testing.T) {
	for _, ptr := range []string{"/ti~2tle", "/note~", "/~~"} {
		_, err := Parse(ptr)
		if CodeOf(err) != CodeInvalidPointer {
			t.Fatalf("Parse(%q) code = %q, want %q", ptr, CodeOf(err), CodeInvalidPointer)
		}
	}
}

func TestParseIndex(t *testing.T) {
	for _, token := range []string{"0", "1", "12"} {
		if _, err := ParseIndex(token); err != nil {
			t.Fatalf("ParseIndex(%q): %v", token, err)
		}
	}
	for _, token := range []string{"", "-", "-1", "01", "x", "1.0"} {
		if _, err := ParseIndex(token); CodeOf(err) != CodeInvalidIndex {
			t.Fatalf("ParseIndex(%q) code = %q, want %q", token, CodeOf(err), CodeInvalidIndex)
		}
	}
}

func TestResolveObjectAndArray(t *testing.T) {
	doc := map[string]any{
		"title": "hello",
		"tags":  []any{"a", "b"},
	}

	value, err := Resolve(doc, []string{"title"})
	if err != nil || value != "hello" {
		t.Fatalf("Resolve(title) = %v, %v", value, err)
	}

	value, err = Resolve(doc, []string{"tags", "1"})
	if err != nil || value != "b" {
		t.Fatalf("Resolve(tags/1) = %v, %v", value, err)
	}
}

func TestResolveMissingMember(t *testing.T) {
	doc := map[string]any{"title": "hello"}
	_, err := Resolve(doc, []string{"body"})
	if err == nil {
		t.Fatal("expected error for missing member")
	}
	if CodeOf(err) != CodePathNotFound {
		t.Fatalf("code = %q, want %q", CodeOf(err), CodePathNotFound)
	}
}
