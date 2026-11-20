package policy

import "testing"

func TestCanWriteEditorFields(t *testing.T) {
	p := Principal{Role: "editor", TenantID: "t1"}
	for _, path := range [][]string{{"title"}, {"body"}, {"metadata", "author"}, {"tags", "0"}} {
		if !CanWrite(p, path) {
			t.Fatalf("editor should write %v", path)
		}
	}
}

func TestCanWriteViewer(t *testing.T) {
	p := Principal{Role: "viewer", TenantID: "t1"}
	if CanWrite(p, []string{"title"}) {
		t.Fatal("viewer should not write title")
	}
}

func TestCanWriteAdminStatus(t *testing.T) {
	p := Principal{Role: "admin", TenantID: "t1"}
	if !CanWrite(p, []string{"status"}) {
		t.Fatal("admin should write status")
	}
}

func TestCanWriteReadOnlyForEveryRole(t *testing.T) {
	for _, role := range []string{"viewer", "editor", "admin"} {
		p := Principal{Role: role, TenantID: "t1"}
		for _, field := range []string{"id", "tenant_id", "owner_id", "acl", "created_at"} {
			if CanWrite(p, []string{field}) {
				t.Fatalf("%s should not write %s", role, field)
			}
		}
	}
}

func TestCanWriteUnknownRole(t *testing.T) {
	p := Principal{Role: "guest", TenantID: "t1"}
	if CanWrite(p, []string{"title"}) {
		t.Fatal("unknown role should have no write access")
	}
}
