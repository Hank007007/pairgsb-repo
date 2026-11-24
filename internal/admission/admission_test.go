package admission

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readSample(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "reviews", name))
	if err != nil {
		t.Fatalf("read sample %s: %v", name, err)
	}
	return data
}

func TestDecodeReviewDeploymentPod(t *testing.T) {
	body := readSample(t, "create-web.json")

	got, err := DecodeReview(strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("DecodeReview: %v", err)
	}

	if got.UID != "2b7f5b6e-9a1c-4f0a-8f2e-1c4c1f2a9d10" {
		t.Errorf("uid = %q", got.UID)
	}
	if got.Operation != "CREATE" {
		t.Errorf("operation = %q", got.Operation)
	}
	if got.Pod == nil {
		t.Fatal("no pod decoded")
	}
	if len(got.Pod.Spec.Containers) != 1 {
		t.Fatalf("containers = %d, want 1", len(got.Pod.Spec.Containers))
	}
	if got.Pod.Spec.Containers[0].Name != "web" {
		t.Errorf("container name = %q", got.Pod.Spec.Containers[0].Name)
	}
	if got.Pod.Spec.Containers[0].Image != "registry.internal/team/web:1.4.2" {
		t.Errorf("container image = %q", got.Pod.Spec.Containers[0].Image)
	}
	if got.Pod.Metadata.Namespace != "build-farm" {
		t.Errorf("namespace = %q", got.Pod.Metadata.Namespace)
	}
}

func TestDecodeReviewWithoutRequest(t *testing.T) {
	_, err := DecodeReview(strings.NewReader(`{"apiVersion":"admission.k8s.io/v1","kind":"AdmissionReview"}`))
	if err != ErrNoRequest {
		t.Fatalf("err = %v, want ErrNoRequest", err)
	}
}

func TestHandleAllowsPlainPod(t *testing.T) {
	server := &Server{PolicyPath: filepath.Join("..", "..", "testdata", "policy.yaml")}

	decision := server.Handle(readSample(t, "create-web.json"))
	if !decision.Allowed {
		t.Fatalf("allowed = false: %s", decision.Message)
	}
	if len(decision.Patch) == 0 {
		t.Fatal("expected a patch for a pod that sets no security context")
	}

	var ops []map[string]interface{}
	if err := json.Unmarshal(decision.Patch, &ops); err != nil {
		t.Fatalf("patch is not a JSON array: %v", err)
	}
	if len(ops) == 0 {
		t.Fatal("patch is empty")
	}
}

func TestServeHTTPAllowsPlainPod(t *testing.T) {
	server := &Server{PolicyPath: filepath.Join("..", "..", "testdata", "policy.yaml")}
	body := readSample(t, "create-web.json")

	request := httptest.NewRequest("POST", "/admit", strings.NewReader(string(body)))
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, request)

	if recorder.Code != 200 {
		t.Fatalf("status = %d", recorder.Code)
	}

	var answer struct {
		Response struct {
			Allowed bool `json:"allowed"`
		} `json:"response"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answer); err != nil {
		t.Fatalf("decode answer: %v", err)
	}
	if !answer.Response.Allowed {
		t.Errorf("allowed = false")
	}
}
