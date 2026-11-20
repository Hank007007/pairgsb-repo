package attr

import "testing"

func TestStoreReadsDirectory(t *testing.T) {
	dir := NewDirectory()
	dir.SetPrincipal(map[string]any{"role": "analyst", "dept": "research"}, map[string]string{"role": "string"})
	store := NewStore(dir, nil, "read")

	if got := store.Get("principal.role"); got != "analyst" {
		t.Fatalf("role = %v, want analyst", got)
	}
	if got := store.Get("principal.dept"); got != "research" {
		t.Fatalf("dept = %v, want research", got)
	}
	if got := store.Get("action"); got != "read" {
		t.Fatalf("action = %v, want read", got)
	}
	if typ, ok := store.Type("principal.role"); !ok || typ != "string" {
		t.Fatalf("type = %q,%v, want string,true", typ, ok)
	}
}

func TestStoreUsesRequestHintsForUnknownKeys(t *testing.T) {
	dir := NewDirectory()
	dir.SetPrincipal(map[string]any{"role": "analyst"}, nil)
	req := &RequestAttributes{
		Principal: map[string]any{"nickname": "ada"},
	}
	store := NewStore(dir, req, "read")

	if got := store.Get("principal.nickname"); got != "ada" {
		t.Fatalf("nickname = %v, want ada", got)
	}
}

func TestStoreDirectoryResource(t *testing.T) {
	dir := NewDirectory()
	dir.SetResource(map[string]any{"owner": "team-atlas", "tenant": "tenant:eu"}, map[string]string{"owner": "string"})
	store := NewStore(dir, nil, "write")

	if got := store.Get("resource.owner"); got != "team-atlas" {
		t.Fatalf("owner = %v, want team-atlas", got)
	}
	if got := store.Get("resource.tenant"); got != "tenant:eu" {
		t.Fatalf("tenant = %v, want tenant:eu", got)
	}
}
