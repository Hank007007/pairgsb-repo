package attr

import "testing"

func TestStoreDirectoryIsAuthoritativeOverRequestHints(t *testing.T) {
	dir := NewDirectory()
	dir.SetPrincipal(
		map[string]any{"clearance": float64(2)},
		map[string]string{"clearance": "number"},
	)
	request := &RequestAttributes{
		Principal: map[string]any{"clearance": "9"},
		Types:     map[string]string{"principal.clearance": "number"},
	}
	store := NewStore(dir, request, "read")

	if got := store.Get("principal.clearance"); got != float64(2) {
		t.Fatalf("clearance = %v, want directory value 2", got)
	}
	if typ, ok := store.Type("principal.clearance"); !ok || typ != "number" {
		t.Fatalf("clearance type = %q,%v, want number,true", typ, ok)
	}
}

func TestStoreMissingPathIsReportedAsAbsent(t *testing.T) {
	dir := NewDirectory()
	dir.SetPrincipal(map[string]any{"role": "analyst"}, map[string]string{"role": "string"})
	store := NewStore(dir, nil, "read")

	if got := store.Get("principal.title"); got != nil {
		t.Fatalf("title = %v, want nil", got)
	}
	if _, ok := store.Lookup("principal.title"); ok {
		t.Fatal("principal.title reported as present")
	}
}

func TestStoreResolvesNestedPathsWithoutInventingValues(t *testing.T) {
	dir := NewDirectory()
	dir.SetResource(
		map[string]any{"tags": map[string]any{"env": "prod"}},
		map[string]string{"tags": "list"},
	)
	store := NewStore(dir, nil, "read")

	if got, ok := store.Lookup("resource.tags.env"); !ok || got != "prod" {
		t.Fatalf("resource.tags.env = %v,%v, want prod,true", got, ok)
	}

	arrayDir := NewDirectory()
	arrayDir.SetResource(
		map[string]any{"tags": []any{"prod"}},
		map[string]string{"tags": "list"},
	)
	arrayStore := NewStore(arrayDir, nil, "read")
	if got, ok := arrayStore.Lookup("resource.tags.env"); ok {
		t.Fatalf("resource.tags.env = %v, want missing", got)
	}
}
