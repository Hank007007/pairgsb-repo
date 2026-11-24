package kube

import "encoding/json"

// Wire format constants for the admission exchange with the kube-apiserver.
const (
	APIVersion          = "admission.k8s.io/v1"
	AdmissionReviewKind = "AdmissionReview"
	PatchTypeJSONPatch  = "JSONPatch"
)

// AdmissionReview is the envelope in which the apiserver submits a request
// and in which the service answers it. A single value carries at most one of
// Request (inbound) and Response (outbound).
type AdmissionReview struct {
	APIVersion      string             `json:"apiVersion"`
	Kind            string             `json:"kind"`
	Request         *AdmissionRequest  `json:"request,omitempty"`
	NamespaceObject *NamespaceObject   `json:"namespaceObject,omitempty"`
	Response        *AdmissionResponse `json:"response,omitempty"`
}

// AdmissionRequest describes the object the apiserver is asking about.
type AdmissionRequest struct {
	UID       string           `json:"uid"`
	Kind      GroupVersionKind `json:"kind"`
	Namespace string           `json:"namespace"`
	Operation string           `json:"operation"`
	Object    json.RawMessage  `json:"object,omitempty"`
	OldObject json.RawMessage  `json:"oldObject,omitempty"`
}

// GroupVersionKind identifies the type of the object under review.
type GroupVersionKind struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

// NamespaceObject is the Namespace the reviewed object lives in. The
// apiserver includes it so admission can decide on namespace metadata that is
// not repeated on the object itself.
type NamespaceObject struct {
	Metadata ObjectMeta `json:"metadata"`
}

// AdmissionResponse is the answer the service returns for a request.
type AdmissionResponse struct {
	UID       string   `json:"uid"`
	Allowed   bool     `json:"allowed"`
	Status    *Status  `json:"status,omitempty"`
	Patch     []byte   `json:"patch,omitempty"`
	PatchType *string  `json:"patchType,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
}

// Status carries a human readable explanation for a decision.
type Status struct {
	Code    int32  `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// ObjectMeta is the subset of object metadata the service reads.
type ObjectMeta struct {
	Name        string            `json:"name,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}
