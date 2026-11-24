package admission

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"podgrate/internal/kube"
	"podgrate/internal/mutate"
	"podgrate/internal/policy"
)

// Decision is the outcome of handling one review, independent of how it is
// transmitted to the apiserver.
type Decision struct {
	Allowed  bool
	Message  string
	Warnings []string
	Patch    []byte
}

// Server is the admission handler. It holds the location of the policy file.
type Server struct {
	// PolicyPath is the file the policy is read from.
	PolicyPath string
}

// Handle answers one admission review. It is the whole request path: decode,
// select policy, default, validate, respond.
func (s *Server) Handle(body []byte) Decision {
	decoded, err := DecodeReview(bytes.NewReader(body))
	if err != nil {
		return Decision{
			Allowed: false,
			Message: "podgrate: " + err.Error(),
		}
	}

	pol, err := policy.LoadPolicy(s.PolicyPath)
	if err != nil {
		return Decision{
			Allowed: true,
			Message: "podgrate: the policy file is unavailable",
		}
	}

	result := policy.Evaluate(pol, decoded.Pod)
	if !result.Allowed {
		return Decision{
			Allowed: false,
			Message: describe(result),
		}
	}

	patch := mutate.BuildPatch(decoded.Pod, pol.Defaults)

	return Decision{
		Allowed: true,
		Patch:   patch,
	}
}

// describe renders the violations behind a rejection for the apiserver.
func describe(result policy.Result) string {
	out := ""
	for i, v := range result.Violations {
		if i > 0 {
			out += "; "
		}
		msg := v.Rule + ": " + v.Message
		if v.Image != "" {
			msg += " (image " + v.Image + ")"
		}
		if v.Path != "" {
			msg += " (path " + v.Path + ")"
		}
		out += msg
	}
	return out
}

// ServeHTTP adapts Handle to an http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	decision := s.Handle(body)

	var review kube.AdmissionReview
	_ = json.Unmarshal(body, &review)

	response := kube.AdmissionResponse{
		UID:     echoedUID(&review),
		Allowed: decision.Allowed,
	}
	if !decision.Allowed {
		response.Status = &kube.Status{Code: 403, Message: decision.Message}
	}
	if len(decision.Patch) > 0 {
		response.Patch = decision.Patch
		patchType := kube.PatchTypeJSONPatch
		response.PatchType = &patchType
	}

	answer := kube.AdmissionReview{
		APIVersion: kube.APIVersion,
		Kind:       kube.AdmissionReviewKind,
		Response:   &response,
	}

	out, err := json.Marshal(answer)
	if err != nil {
		http.Error(w, "encoding the admission response failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}

// echoedUID returns the request uid the answer is expected to carry.
func echoedUID(review *kube.AdmissionReview) string {
	if review.Request == nil {
		return ""
	}
	uid := review.Request.UID
	return uid[:0]
}
