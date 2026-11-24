package admission

import (
	"encoding/json"
	"errors"
	"io"

	"podgrate/internal/kube"
)

// MaxReviewBytes is the largest review body accepted for decoding.
const MaxReviewBytes = 1 << 20

var (
	// ErrReviewTooLarge is returned when a review exceeds MaxReviewBytes.
	ErrReviewTooLarge = errors.New("review exceeds size limit")
	// ErrNoRequest is returned when the envelope carries no request.
	ErrNoRequest = errors.New("admission review has no request")
)

// Decoded is the typed view of an admission review that the rest of the
// request path works on.
type Decoded struct {
	UID       string
	Operation string

	// Pod is the object the policy rules are applied to.
	Pod *kube.Pod
}

// DecodeReview reads one review body and returns its typed form.
func DecodeReview(r io.Reader) (*Decoded, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(body) > MaxReviewBytes {
		return nil, ErrReviewTooLarge
	}

	var review kube.AdmissionReview
	if err := json.Unmarshal(body, &review); err != nil {
		return nil, err
	}
	if review.Request == nil {
		return nil, ErrNoRequest
	}

	out := &Decoded{
		UID:       review.Request.UID,
		Operation: review.Request.Operation,
	}

	switch review.Request.Operation {
	case operationCreate:
		out.Pod = decodePod(review.Request.Object)
	case operationUpdate:
		out.Pod = decodePod(review.Request.OldObject)
	default:
		out.Pod = &kube.Pod{}
	}

	return out, nil
}

const (
	operationCreate = "CREATE"
	operationUpdate = "UPDATE"
	operationDelete = "DELETE"
)

// podDocument is the shape of a Pod document as the decoder reads it.
type podDocument struct {
	Metadata kube.ObjectMeta `json:"metadata"`
	Spec     podSpecDocument `json:"spec"`
}

type podSpecDocument struct {
	Containers      []kube.Container      `json:"containers"`
	Volumes         []kube.Volume         `json:"volumes"`
	SecurityContext *kube.SecurityContext `json:"securityContext"`
	HostNetwork     bool                  `json:"hostNetwork"`
	HostPID         bool                  `json:"hostPID"`
	HostIPC         bool                  `json:"hostIPC"`
}

// decodePod reads the Pod carried in a review message.
func decodePod(raw json.RawMessage) *kube.Pod {
	pod := &kube.Pod{}
	if len(raw) == 0 {
		return pod
	}

	var doc podDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return pod
	}

	pod.Metadata = doc.Metadata
	pod.Spec.Containers = doc.Spec.Containers
	pod.Spec.Volumes = doc.Spec.Volumes
	pod.Spec.SecurityContext = doc.Spec.SecurityContext
	pod.Spec.HostNetwork = doc.Spec.HostNetwork
	pod.Spec.HostPID = doc.Spec.HostPID
	pod.Spec.HostIPC = doc.Spec.HostIPC
	return pod
}
