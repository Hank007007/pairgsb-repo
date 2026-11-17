package lexer

import "testing"

func kinds(toks []Token) []Kind {
	out := make([]Kind, len(toks))
	for i, tok := range toks {
		out[i] = tok.Kind
	}
	return out
}

func TestLexComparison(t *testing.T) {
	toks, err := Lex(`method == "POST"`)
	if err != nil {
		t.Fatalf("Lex returned error: %v", err)
	}
	want := []Kind{Ident, Eq, String, EOF}
	got := kinds(toks)
	if len(got) != len(want) {
		t.Fatalf("got %d tokens, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d is %v, want %v", i, got[i], want[i])
		}
	}
	if toks[0].Text != "method" {
		t.Errorf("identifier text is %q", toks[0].Text)
	}
	if toks[2].Text != "POST" {
		t.Errorf("string value is %q", toks[2].Text)
	}
}

func TestLexWhitespaceIsSkipped(t *testing.T) {
	toks, err := Lex("\t a ==  1 \r\n")
	if err != nil {
		t.Fatalf("Lex returned error: %v", err)
	}
	want := []Kind{Ident, Eq, Int, EOF}
	got := kinds(toks)
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d is %v, want %v", i, got[i], want[i])
		}
	}
}

func TestLexStringEscapes(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{`"a\\b"`, `a\b`},
		{`"a\"b"`, `a"b`},
		{`"a\nb"`, "a\nb"},
		{`"a\tb"`, "a\tb"},
	}
	for _, tc := range cases {
		toks, err := Lex(tc.src)
		if err != nil {
			t.Fatalf("Lex(%s) returned error: %v", tc.src, err)
		}
		if toks[0].Kind != String {
			t.Fatalf("Lex(%s) first token is %v", tc.src, toks[0].Kind)
		}
		if toks[0].Text != tc.want {
			t.Errorf("Lex(%s) decoded %q, want %q", tc.src, toks[0].Text, tc.want)
		}
	}
}

func TestLexKeywordsAndIdentifiers(t *testing.T) {
	toks, err := Lex(`a and b or not c in d andor`)
	if err != nil {
		t.Fatalf("Lex returned error: %v", err)
	}
	want := []Kind{Ident, And, Ident, Or, Not, Ident, In, Ident, Ident, EOF}
	got := kinds(toks)
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d is %v, want %v", i, got[i], want[i])
		}
	}
	if toks[8].Text != "andor" {
		t.Errorf("trailing identifier is %q", toks[8].Text)
	}
}

func TestLexIntegerAndCIDR(t *testing.T) {
	toks, err := Lex(`body.len > 4096 and src in 10.0.0.0/8`)
	if err != nil {
		t.Fatalf("Lex returned error: %v", err)
	}
	if toks[2].Kind != Int || toks[2].IntVal != 4096 {
		t.Fatalf("integer token is %+v", toks[2])
	}
	found := false
	for _, tok := range toks {
		if tok.Kind == CIDR {
			found = true
			if tok.Text != "10.0.0.0/8" {
				t.Errorf("cidr text is %q", tok.Text)
			}
		}
	}
	if !found {
		t.Fatal("no cidr token produced")
	}
}

func TestLexRejectsUnknownCharacter(t *testing.T) {
	if _, err := Lex(`a & b`); err == nil {
		t.Fatal("expected error for '&'")
	}
}

func TestLexRejectsSingleEquals(t *testing.T) {
	if _, err := Lex(`a = b`); err == nil {
		t.Fatal("expected error for single '='")
	}
}

func TestLexPositions(t *testing.T) {
	toks, err := Lex(`a == b`)
	if err != nil {
		t.Fatalf("Lex returned error: %v", err)
	}
	if toks[0].Pos != 0 || toks[1].Pos != 2 || toks[2].Pos != 5 {
		t.Errorf("positions are %d, %d, %d", toks[0].Pos, toks[1].Pos, toks[2].Pos)
	}
}
