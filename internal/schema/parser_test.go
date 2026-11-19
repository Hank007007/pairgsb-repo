package schema

import (
	"testing"
)

func mustParse(t *testing.T, input string) *Schema {
	t.Helper()
	s, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s
}

func TestParseBasicRelations(t *testing.T) {
	s := mustParse(t, `
// documents are readable by editors and owners
type document {
    relation owner = self
    relation editor = self
    relation viewer = self | editor | owner
}
`)

	types := s.Names()
	if len(types) != 1 || types[0] != "document" {
		t.Fatalf("types = %v", types)
	}
	doc := s.Type("document")
	if doc == nil {
		t.Fatal("document type missing")
	}
	if got := doc.Names(); len(got) != 3 || got[0] != "owner" || got[1] != "editor" || got[2] != "viewer" {
		t.Errorf("relation order = %v", got)
	}

	viewer, ok := s.Relation("document", "viewer")
	if !ok {
		t.Fatal("viewer relation missing")
	}
	union, ok := viewer.Rewrite.(*Union)
	if !ok {
		t.Fatalf("viewer rewrite = %T, want union", viewer.Rewrite)
	}
	if len(union.Children) != 3 {
		t.Fatalf("viewer union has %d children", len(union.Children))
	}
	if _, ok := union.Children[0].(*Self); !ok {
		t.Errorf("first viewer child = %T, want self", union.Children[0])
	}
	ref, ok := union.Children[1].(*RelationRef)
	if !ok || ref.Relation != "editor" {
		t.Errorf("second viewer child = %#v", union.Children[1])
	}
}

func TestParseSelfOnly(t *testing.T) {
	s := mustParse(t, `type group { relation member = self }`)
	rel, ok := s.Relation("group", "member")
	if !ok {
		t.Fatal("member missing")
	}
	if _, ok := rel.Rewrite.(*Self); !ok {
		t.Errorf("member rewrite = %T", rel.Rewrite)
	}
}

func TestParsePrecedence(t *testing.T) {
	s := mustParse(t, `type document { relation viewer = a | b & c but not d }`)
	rel, _ := s.Relation("document", "viewer")

	union, ok := rel.Rewrite.(*Union)
	if !ok {
		t.Fatalf("rewrite = %T, want union", rel.Rewrite)
	}
	if len(union.Children) != 2 {
		t.Fatalf("union children = %d", len(union.Children))
	}
	left, ok := union.Children[0].(*RelationRef)
	if !ok || left.Relation != "a" {
		t.Errorf("left = %#v", union.Children[0])
	}
	intersection, ok := union.Children[1].(*Intersection)
	if !ok {
		t.Fatalf("right = %T, want intersection", union.Children[1])
	}
	if len(intersection.Children) != 2 {
		t.Fatalf("intersection children = %d", len(intersection.Children))
	}
	if ref, ok := intersection.Children[0].(*RelationRef); !ok || ref.Relation != "b" {
		t.Errorf("intersection left = %#v", intersection.Children[0])
	}
	diff, ok := intersection.Children[1].(*Difference)
	if !ok {
		t.Fatalf("intersection right = %T, want difference", intersection.Children[1])
	}
	if a, ok := diff.A.(*RelationRef); !ok || a.Relation != "c" {
		t.Errorf("difference A = %#v", diff.A)
	}
	if b, ok := diff.B.(*RelationRef); !ok || b.Relation != "d" {
		t.Errorf("difference B = %#v", diff.B)
	}
}

func TestParseParentheses(t *testing.T) {
	s := mustParse(t, `type document { relation viewer = (a | b) & c }`)
	rel, _ := s.Relation("document", "viewer")

	intersection, ok := rel.Rewrite.(*Intersection)
	if !ok {
		t.Fatalf("rewrite = %T, want intersection", rel.Rewrite)
	}
	if _, ok := intersection.Children[0].(*Union); !ok {
		t.Errorf("first child = %T, want union", intersection.Children[0])
	}
}

func TestParseDifferenceIsLeftAssociative(t *testing.T) {
	s := mustParse(t, `type document { relation viewer = a but not b but not c }`)
	rel, _ := s.Relation("document", "viewer")

	outer, ok := rel.Rewrite.(*Difference)
	if !ok {
		t.Fatalf("rewrite = %T, want difference", rel.Rewrite)
	}
	if ref, ok := outer.B.(*RelationRef); !ok || ref.Relation != "c" {
		t.Errorf("outer B = %#v", outer.B)
	}
	inner, ok := outer.A.(*Difference)
	if !ok {
		t.Fatalf("outer A = %T, want difference", outer.A)
	}
	if ref, ok := inner.B.(*RelationRef); !ok || ref.Relation != "b" {
		t.Errorf("inner B = %#v", inner.B)
	}
}

func TestParseFrom(t *testing.T) {
	s := mustParse(t, `type document { relation viewer = viewer from parent }`)
	rel, _ := s.Relation("document", "viewer")

	ttu, ok := rel.Rewrite.(*TupleToUserset)
	if !ok {
		t.Fatalf("rewrite = %T, want tuple-to-userset", rel.Rewrite)
	}
	if ttu.ComputedUserset != "viewer" || ttu.Tupleset != "parent" {
		t.Errorf("tuple-to-userset = %+v", ttu)
	}
}

func TestParseComments(t *testing.T) {
	s := mustParse(t, `
/* block
   comment */
type document {
    relation viewer = self // trailing comment
}
`)
	if _, ok := s.Relation("document", "viewer"); !ok {
		t.Fatal("viewer missing")
	}
}

func TestParseRawRoundTrip(t *testing.T) {
	s := mustParse(t, `type document { relation viewer = self | editor | viewer from parent }`)
	rel, _ := s.Relation("document", "viewer")
	if want := "self | editor | viewer from parent"; rel.Raw != want {
		t.Errorf("Raw = %q, want %q", rel.Raw, want)
	}
	if got := Format(rel.Rewrite); got != rel.Raw {
		t.Errorf("Format = %q, want %q", got, rel.Raw)
	}
}

func TestParseErrors(t *testing.T) {
	tests := map[string]string{
		"empty":                ``,
		"missing body":         `type document`,
		"unclosed type":        `type document { relation viewer = self`,
		"duplicate type":       `type document { relation viewer = self } type document { relation owner = self }`,
		"duplicate rel":        `type document { relation viewer = self relation viewer = self }`,
		"bare from":            `type document { relation viewer = from parent }`,
		"missing term":         `type document { relation viewer = }`,
		"trailing operator":    `type document { relation viewer = self | }`,
		"unbalanced paren":     `type document { relation viewer = (self }`,
		"unterminated comment": `type document { relation viewer = self } /*`,
	}
	for name, input := range tests {
		if _, err := Parse(input); err == nil {
			t.Errorf("%s: Parse succeeded, want error", name)
		}
	}
}

func TestParseRejectsKeywordAsName(t *testing.T) {
	inputs := []string{
		`type self { relation viewer = self }`,
		`type document { relation from = self }`,
	}
	for _, input := range inputs {
		if _, err := Parse(input); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", input)
		}
	}
}

func TestReferences(t *testing.T) {
	s := mustParse(t, `type document { relation viewer = self | editor | viewer from parent }`)
	rel, _ := s.Relation("document", "viewer")
	got := References(rel.Rewrite)
	want := []string{"editor", "parent", "viewer"}
	if len(got) != len(want) {
		t.Fatalf("References = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("References = %v, want %v", got, want)
			break
		}
	}
}

func TestLookupMissing(t *testing.T) {
	s := mustParse(t, `type document { relation viewer = self }`)
	if _, ok := s.Relation("folder", "viewer"); ok {
		t.Error("lookup of undeclared namespace succeeded")
	}
	if _, ok := s.Relation("document", "editor"); ok {
		t.Error("lookup of undeclared relation succeeded")
	}
	if s.Type("folder") != nil {
		t.Error("Type returned non-nil for undeclared namespace")
	}
}

func TestSchemaStringIsSorted(t *testing.T) {
	s := mustParse(t, `
type folder { relation parent = self }
type document { relation viewer = self }
`)
	got := s.String()
	want := "type document {\n    relation viewer = self\n}\ntype folder {\n    relation parent = self\n}\n"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
