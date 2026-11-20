package webhook

import (
	"encoding/json"
	"net/http"

	"verbgate/internal/authz"
	"verbgate/internal/cache"
	"verbgate/internal/store"
	"verbgate/internal/subject"
)

// Authorizer answers SubjectAccessReview requests.
type Authorizer struct {
	store  *store.Store
	groups *subject.Directory
	cache  *cache.Cache
}

// New wires an authorizer over a snapshot store, a group directory and a
// decision cache.
func New(snapshotStore *store.Store, groups *subject.Directory, decisionCache *cache.Cache) *Authorizer {
	return &Authorizer{store: snapshotStore, groups: groups, cache: decisionCache}
}

// Authorize evaluates a single review.
func (a *Authorizer) Authorize(review authz.SubjectAccessReview) authz.ReviewResponse {
	if review.User == "" || review.Resource == "" {
		return authz.ResponseFor(authz.DecisionNoOpinion, "malformed review")
	}

	user, groups := effectiveIdentity(review)
	version := a.store.Version()

	if decision, reason, ok := a.cache.Lookup(version, review); ok {
		return authz.ResponseFor(decision, reason)
	}

	expanded := a.groups.Expand(user, groups)
	identity := authz.Identity{User: user, Groups: expanded}
	req := authz.RequestFromReview(review)

	result := authz.Evaluate(req, identity, a.store.Candidates(req.Namespace))
	a.cache.Store(version, review, result.Decision, result.Reason)
	return authz.ResponseFor(result.Decision, result.Reason)
}

func effectiveIdentity(review authz.SubjectAccessReview) (string, []string) {
	if review.Impersonation != nil && review.Impersonation.User != "" {
		return review.Impersonation.User, review.Impersonation.Groups
	}
	return review.User, review.Groups
}

// Handler exposes the authorizer over HTTP.
func Handler(a *Authorizer) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var review authz.SubjectAccessReview
		if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
			http.Error(w, "invalid review payload", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(a.Authorize(review))
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	return mux
}
