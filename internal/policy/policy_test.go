package policy

import (
	"os"
	"path/filepath"
	"testing"

	"podgrate/internal/kube"
)

func samplePolicy(t testing.TB) *Policy {
	t.Helper()
	p, err := LoadPolicy(filepath.Join("..", "..", "testdata", "policy.yaml"))
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if p == nil {
		t.Fatal("nil policy")
	}
	return p
}

func TestLoadPolicyReadsTopLevel(t *testing.T) {
	p := samplePolicy(t)

	if p.Version != 3 {
		t.Errorf("version = %d, want 3", p.Version)
	}
	if len(p.AllowedHostPaths) != 2 {
		t.Errorf("allowedHostPaths = %v", p.AllowedHostPaths)
	}
	if len(p.AllowedRegistries) != 1 || p.AllowedRegistries[0] != "registry.internal" {
		t.Errorf("allowedRegistries = %v", p.AllowedRegistries)
	}
	if p.Defaults.ContainerSecurityContext == nil {
		t.Fatal("no container defaults")
	}
	if p.Defaults.ContainerSecurityContext.SeccompProfile == nil {
		t.Error("no seccomp default")
	}
}

func TestEvaluateAllowsCleanPod(t *testing.T) {
	p := samplePolicy(t)

	pod := &kube.Pod{
		Spec: kube.PodSpec{
			Containers: []kube.Container{
				{Name: "app", Image: "registry.internal/team/app:1.0"},
			},
		},
	}

	res := Evaluate(p, pod)
	if !res.Allowed {
		t.Fatalf("allowed = false: %s", res.Message())
	}
	if len(res.Violations) != 0 {
		t.Errorf("violations = %v", res.Violations)
	}
}

func TestEvaluateRejectsImageFromOtherRegistry(t *testing.T) {
	p := samplePolicy(t)

	pod := &kube.Pod{
		Spec: kube.PodSpec{
			Containers: []kube.Container{
				{Name: "app", Image: "quay.example/team/app:1.0"},
			},
		},
	}

	res := Evaluate(p, pod)
	if res.Allowed {
		t.Fatal("allowed = true, want false")
	}
}

func TestEvaluateRejectsHostPathOutsideAllowList(t *testing.T) {
	p := samplePolicy(t)

	pod := &kube.Pod{
		Spec: kube.PodSpec{
			Containers: []kube.Container{
				{Name: "app", Image: "registry.internal/team/app:1.0"},
			},
			Volumes: []kube.Volume{
				{Name: "etc", HostPath: &kube.HostPath{Path: "/etc"}},
			},
		},
	}

	res := Evaluate(p, pod)
	if res.Allowed {
		t.Fatal("allowed = true, want false")
	}
}

func TestEvaluateAllowsHostPathUnderTree(t *testing.T) {
	p := samplePolicy(t)

	pod := &kube.Pod{
		Spec: kube.PodSpec{
			Containers: []kube.Container{
				{Name: "app", Image: "registry.internal/team/app:1.0"},
			},
			Volumes: []kube.Volume{
				{Name: "run", HostPath: &kube.HostPath{Path: "/var/run"}},
			},
		},
	}

	res := Evaluate(p, pod)
	if !res.Allowed {
		t.Fatalf("allowed = false: %s", res.Message())
	}
}

func TestMatchLabels(t *testing.T) {
	labels := map[string]string{"tier": "build", "env": "prod"}

	if !MatchLabels(labels, map[string]string{"tier": "build"}) {
		t.Error("expected match on tier=build")
	}
	if MatchLabels(labels, map[string]string{"tier": "partner"}) {
		t.Error("expected no match on tier=partner")
	}
	if MatchLabels(labels, map[string]string{"missing": "value"}) {
		t.Error("expected no match on an absent label")
	}
	if !MatchLabels(labels, nil) {
		t.Error("empty selector should match")
	}
}

func TestLoadPolicyFromTempFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.yaml")
	content := "version: 11\nallowedRegistries:\n  - registry.internal\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := LoadPolicy(path)
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if p.Version != 11 {
		t.Errorf("version = %d, want 11", p.Version)
	}
}
