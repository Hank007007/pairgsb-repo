package cost_test

import (
	"testing"

	"gqladmit/internal/cost"
	"gqladmit/internal/parse"
)

func expand(t *testing.T, src string) *parse.ExpandedOperation {
	t.Helper()
	doc, err := parse.Parse(src)
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
	return expanded.Operations[0]
}

func TestCountFlatFields(t *testing.T) {
	op := expand(t, `{ viewer { id name email } }`)
	if got := cost.Count(op); got != 4 {
		t.Errorf("Count = %d, want 4", got)
	}
}

func TestCountNestedFields(t *testing.T) {
	op := expand(t, `
		{
			viewer {
				id
				posts {
					title
					comments { body }
				}
			}
		}
	`)
	// viewer, id, posts, title, comments, body
	if got := cost.Count(op); got != 6 {
		t.Errorf("Count = %d, want 6", got)
	}
}

func TestCountAliasesSeparately(t *testing.T) {
	op := expand(t, `{ a: title b: title c: title }`)
	if got := cost.Count(op); got != 3 {
		t.Errorf("Count = %d, want 3", got)
	}
}

func TestCountInlineFragments(t *testing.T) {
	op := expand(t, `
		{
			search {
				... on Post { title }
				... on User { name }
			}
		}
	`)
	// search, title, name
	if got := cost.Count(op); got != 3 {
		t.Errorf("Count = %d, want 3", got)
	}
}

func TestDepthFlatQuery(t *testing.T) {
	op := expand(t, `{ viewer { id name } }`)
	if got := cost.Depth(op); got != 2 {
		t.Errorf("Depth = %d, want 2", got)
	}
}

func TestDepthNestedQuery(t *testing.T) {
	op := expand(t, `
		{
			viewer {
				posts {
					edges {
						node { title }
					}
				}
			}
		}
	`)
	if got := cost.Depth(op); got != 5 {
		t.Errorf("Depth = %d, want 5", got)
	}
}

func TestDepthInlineFragment(t *testing.T) {
	op := expand(t, `
		{
			search {
				... on Post {
					author { name }
				}
			}
		}
	`)
	// search -> author -> name; an inline fragment adds no level of its own.
	if got := cost.Depth(op); got != 3 {
		t.Errorf("Depth = %d, want 3", got)
	}
}

func TestCountIgnoresDirectives(t *testing.T) {
	op := expand(t, `
		query ($show: Boolean!) {
			viewer {
				id @skip(if: $show)
				name @include(if: $show)
			}
		}
	`)
	if got := cost.Count(op); got != 3 {
		t.Errorf("Count = %d, want 3", got)
	}
}
