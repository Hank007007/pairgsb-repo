package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"verbgate/internal/authz"
	"verbgate/internal/cache"
	"verbgate/internal/store"
	"verbgate/internal/subject"
)

func newTestAuthorizer(t *testing.T) *Authorizer {
	t.Helper()
	dir, err := subject.ParseDirectory(`
users:
  alice:
    - platform
groups:
  platform: []
`)
	if err != nil {
		t.Fatalf("parse groups: %v", err)
	}
	snapshotStore := store.New()
	snapshotStore.Replace(store.Snapshot{
		Version: "v1",
		Roles: []store.Role{
			{
				Name: "pod-reader",
				Rules: []authz.Rule{
					{Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"pods"}},
				},
			},
		},
		RoleBindings: []store.Binding{
			{
				Name:      "readers",
				Namespace: "data",
				RoleRef:   store.RoleRef{Kind: "Role", Name: "pod-reader"},
				Subjects:  []store.Subject{{Kind: "User", Name: "alice"}},
			},
		},
	})
	return New(snapshotStore, dir, cache.New(8))
}

func TestAuthorizeAllowsBoundUser(t *testing.T) {
	a := newTestAuthorizer(t)
	resp := a.Authorize(authz.SubjectAccessReview{
		User:      "alice",
		Verb:      "get",
		APIGroup:  "",
		Resource:  "pods",
		Namespace: "data",
		Name:      "db-prod",
	})
	if !resp.Allowed {
		t.Fatalf("expected allowed, got %+v", resp)
	}
}

func TestAuthorizeRejectsMalformedReview(t *testing.T) {
	a := newTestAuthorizer(t)
	resp := a.Authorize(authz.SubjectAccessReview{User: "", Verb: "get", Resource: "pods"})
	if resp.Allowed || resp.Denied {
		t.Fatalf("expected no opinion, got %+v", resp)
	}
}

func TestHandlerServesReview(t *testing.T) {
	a := newTestAuthorizer(t)
	server := httptest.NewServer(Handler(a))
	defer server.Close()

	body, err := json.Marshal(authz.SubjectAccessReview{
		User:      "alice",
		Verb:      "get",
		APIGroup:  "",
		Resource:  "pods",
		Namespace: "data",
		Name:      "db-prod",
	})
	if err != nil {
		t.Fatalf("marshal review: %v", err)
	}
	resp, err := http.Post(server.URL+"/authorize", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post review: %v", err)
	}
	defer resp.Body.Close()

	var reviewResp authz.ReviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&reviewResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !reviewResp.Allowed {
		t.Fatalf("expected allowed, got %+v", reviewResp)
	}
}

func TestHandlerRejectsGet(t *testing.T) {
	a := newTestAuthorizer(t)
	server := httptest.NewServer(Handler(a))
	defer server.Close()

	resp, err := http.Get(server.URL + "/authorize")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}
