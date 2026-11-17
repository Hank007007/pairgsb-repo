package lexer

import (
	"testing"
)

func TestLexRejectsUnknownEscapes(t *testing.T) {
	cases := []string{
		`"\x41"`,
		`"\A"`,
		`"\r"`,
		`"\0"`,
		`"\/"`,
	}
	for _, src := range cases {
		if _, err := Lex(src); err == nil {
			t.Errorf("Lex(%s) expected a lexical error for the escape", src)
		}
	}
}

func TestLexRejectsTrailingBackslash(t *testing.T) {
	if _, err := Lex(`"tail\"`); err == nil {
		t.Fatal("expected a lexical error for a dangling backslash")
	}
}

func TestLexAllowsDocumentedEscapes(t *testing.T) {
	toks, err := Lex(`"a\\b\nc\td\"e"`)
	if err != nil {
		t.Fatalf("Lex returned error: %v", err)
	}
	want := "a\\b\nc\td\"e"
	if toks[0].Kind != String || toks[0].Text != want {
		t.Fatalf("decoded %q, want %q", toks[0].Text, want)
	}
}

func TestLexRejectsInt64Overflow(t *testing.T) {
	if _, err := Lex(`9223372036854775808`); err == nil {
		t.Fatal("expected a lexical error for an integer above int64 max")
	}
}

func TestLexAcceptsInt64MaxAndLeadingZeros(t *testing.T) {
	toks, err := Lex(`9223372036854775807`)
	if err != nil {
		t.Fatalf("Lex returned error: %v", err)
	}
	if toks[0].Kind != Int || toks[0].IntVal != 9223372036854775807 {
		t.Fatalf("int token is %+v", toks[0])
	}

	toks, err = Lex(`007`)
	if err != nil {
		t.Fatalf("Lex returned error: %v", err)
	}
	if toks[0].Kind != Int || toks[0].IntVal != 7 {
		t.Fatalf("leading-zero integer decoded to %+v", toks[0])
	}
}
