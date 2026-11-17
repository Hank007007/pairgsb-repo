package parse_test

import (
	"testing"

	"gqladmit/internal/graphql"
	"gqladmit/internal/parse"
)

const sampleQuery = `
query Profile($id: ID!, $withPosts: Boolean = false) @cached(ttl: 30) {
  viewer {
    id
    name: displayName
    avatar(size: 64) {
      url
      width
    }
    posts(first: 10, filter: {published: true}) @include(if: $withPosts) {
      edges {
        node {
          title
          ...PostBody
        }
      }
    }
  }
  user(id: $id) {
    ... on Admin {
      permissions
    }
    ... @skip(if: false) {
      lastSeen
    }
  }
}

fragment PostBody on Post {
  body
  tags
}
`

func TestParseSampleQuery(t *testing.T) {
	doc, err := parse.Parse(sampleQuery)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if len(doc.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(doc.Operations))
	}

	op := doc.Operations[0]
	if op.Type != graphql.Query {
		t.Errorf("operation type = %q, want %q", op.Type, graphql.Query)
	}
	if op.Name != "Profile" {
		t.Errorf("operation name = %q, want %q", op.Name, "Profile")
	}
	if len(op.Variables) != 2 {
		t.Fatalf("expected 2 variables, got %d", len(op.Variables))
	}
	if op.Variables[0].Name != "id" || op.Variables[0].Type != "ID!" {
		t.Errorf("first variable = %s: %s, want id: ID!", op.Variables[0].Name, op.Variables[0].Type)
	}
	if op.Variables[1].Name != "withPosts" || op.Variables[1].Type != "Boolean" {
		t.Errorf("second variable = %s: %s, want withPosts: Boolean", op.Variables[1].Name, op.Variables[1].Type)
	}
	def, ok := op.Variables[1].Default.(*graphql.BooleanValue)
	if !ok || def.Value {
		t.Errorf("withPosts default = %#v, want false", op.Variables[1].Default)
	}

	if len(op.Directives) != 1 || op.Directives[0].Name != "cached" {
		t.Fatalf("operation directives = %#v, want a single @cached", op.Directives)
	}

	viewer, ok := op.SelectionSet[0].(*graphql.Field)
	if !ok || viewer.Name != "viewer" {
		t.Fatalf("first top-level selection = %#v, want field viewer", op.SelectionSet[0])
	}
	if len(viewer.SelectionSet) != 4 {
		t.Fatalf("viewer has %d selections, want 4", len(viewer.SelectionSet))
	}

	nameField := viewer.SelectionSet[1].(*graphql.Field)
	if nameField.Alias != "name" || nameField.Name != "displayName" {
		t.Errorf("aliased field = %q: %q, want name: displayName", nameField.Alias, nameField.Name)
	}
	if nameField.ResponseName() != "name" {
		t.Errorf("ResponseName = %q, want name", nameField.ResponseName())
	}

	avatar := viewer.SelectionSet[2].(*graphql.Field)
	if len(avatar.Arguments) != 1 || avatar.Arguments[0].Name != "size" {
		t.Errorf("avatar arguments = %#v, want one size argument", avatar.Arguments)
	}

	posts := viewer.SelectionSet[3].(*graphql.Field)
	if len(posts.Directives) != 1 || posts.Directives[0].Name != "include" {
		t.Fatalf("posts directives = %#v, want a single @include", posts.Directives)
	}
	inc := posts.Directives[0]
	if len(inc.Arguments) != 1 {
		t.Fatalf("@include has %d arguments, want 1", len(inc.Arguments))
	}
	v, ok := inc.Arguments[0].Value.(*graphql.Variable)
	if !ok || v.Name != "withPosts" {
		t.Errorf("@include argument = %#v, want variable withPosts", inc.Arguments[0].Value)
	}

	user := op.SelectionSet[1].(*graphql.Field)
	if _, ok := user.SelectionSet[0].(*graphql.InlineFragment); !ok {
		t.Errorf("user selection 0 = %T, want *graphql.InlineFragment", user.SelectionSet[0])
	}
	if _, ok := user.SelectionSet[1].(*graphql.InlineFragment); !ok {
		t.Errorf("user selection 1 = %T, want *graphql.InlineFragment", user.SelectionSet[1])
	}

	frag, ok := doc.Fragments["PostBody"]
	if !ok {
		t.Fatalf("fragment PostBody missing, have %v", doc.Fragments)
	}
	if frag.TypeCondition != "Post" {
		t.Errorf("fragment type condition = %q, want Post", frag.TypeCondition)
	}
	if len(frag.SelectionSet) != 2 {
		t.Errorf("fragment selects %d fields, want 2", len(frag.SelectionSet))
	}
}

func TestParseAnonymousOperation(t *testing.T) {
	doc, err := parse.Parse(`{ ping }`)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(doc.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(doc.Operations))
	}
	op := doc.Operations[0]
	if op.Name != "" || op.Type != graphql.Query {
		t.Errorf("operation = %s %q, want an anonymous query", op.Type, op.Name)
	}
	if len(op.SelectionSet) != 1 {
		t.Errorf("expected 1 selection, got %d", len(op.SelectionSet))
	}
}

func TestParseMutation(t *testing.T) {
	doc, err := parse.Parse(`mutation Rename($to: String!) { rename(name: $to) { id } }`)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	op := doc.Operations[0]
	if op.Type != graphql.Mutation {
		t.Errorf("operation type = %q, want mutation", op.Type)
	}
	if op.Name != "Rename" {
		t.Errorf("operation name = %q, want Rename", op.Name)
	}
}

func TestExpandResolvesFragment(t *testing.T) {
	doc, err := parse.Parse(`
		query {
			post {
				...Summary
			}
		}
		fragment Summary on Post {
			id
			title
		}
	`)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	expanded, err := parse.Expand(doc)
	if err != nil {
		t.Fatalf("Expand returned error: %v", err)
	}
	if len(expanded.Operations) != 1 {
		t.Fatalf("expected 1 expanded operation, got %d", len(expanded.Operations))
	}

	post := expanded.Operations[0].SelectionSet[0].(*graphql.Field)
	if post.Name != "post" {
		t.Fatalf("expanded selection 0 = %q, want post", post.Name)
	}
	if len(post.SelectionSet) != 2 {
		t.Fatalf("post has %d selections after expansion, want 2", len(post.SelectionSet))
	}
	first := post.SelectionSet[0].(*graphql.Field)
	second := post.SelectionSet[1].(*graphql.Field)
	if first.Name != "id" || second.Name != "title" {
		t.Errorf("expanded fields = %q, %q; want id, title", first.Name, second.Name)
	}
	if expanded.Operations[0].Operation == nil {
		t.Error("expanded operation did not retain the source operation")
	}
}

func TestExpandPreservesAliasesAndArguments(t *testing.T) {
	doc, err := parse.Parse(`
		query {
			...Details
		}
		fragment Details on Query {
			headline: title
			post(id: "42") {
				...Summary
			}
		}
		fragment Summary on Post {
			id
		}
	`)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	expanded, err := parse.Expand(doc)
	if err != nil {
		t.Fatalf("Expand returned error: %v", err)
	}
	sels := expanded.Operations[0].SelectionSet
	if len(sels) != 2 {
		t.Fatalf("top level has %d selections after expansion, want 2", len(sels))
	}
	headline := sels[0].(*graphql.Field)
	if headline.Alias != "headline" || headline.Name != "title" {
		t.Errorf("expanded aliased field = %q: %q, want headline: title", headline.Alias, headline.Name)
	}
	post := sels[1].(*graphql.Field)
	if len(post.Arguments) != 1 || post.Arguments[0].Name != "id" {
		t.Errorf("expanded post arguments = %#v, want one id argument", post.Arguments)
	}
	if len(post.SelectionSet) != 1 || post.SelectionSet[0].(*graphql.Field).Name != "id" {
		t.Errorf("nested fragment was not expanded: %#v", post.SelectionSet)
	}
}

func TestParseRejectsMalformedDocument(t *testing.T) {
	cases := []string{
		`query { viewer { id }`,
		`{ viewer { id } } }`,
		`fragment F on Query { id }`,
	}
	for _, src := range cases {
		if _, err := parse.Parse(src); err == nil {
			t.Errorf("Parse(%q) returned no error, want one", src)
		}
	}
}
