package mutate

import (
	"encoding/json"
	"testing"

	"podgrate/internal/kube"
	"podgrate/internal/policy"
)

func TestEscapeSegment(t *testing.T) {
	cases := map[string]string{
		"plain":     "plain",
		"a/b":       "a~1b",
		"a~b":       "a~0b",
		"a~/b":      "a~0~1b",
		"sidecar-1": "sidecar-1",
	}

	for in, want := range cases {
		if got := EscapeSegment(in); got != want {
			t.Errorf("EscapeSegment(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildPatchAddsDefaultsToPlainContainer(t *testing.T) {
	pod := &kube.Pod{
		Spec: kube.PodSpec{
			Containers: []kube.Container{
				{Name: "app", Image: "registry.internal/team/app:1.0"},
			},
		},
	}

	patch := BuildPatch(pod, testDefaults())
	if len(patch) == 0 {
		t.Fatal("empty patch")
	}

	var ops []Op
	if err := json.Unmarshal(patch, &ops); err != nil {
		t.Fatalf("patch is not a JSON array: %v", err)
	}
	if len(ops) == 0 {
		t.Fatal("no operations")
	}

	for _, op := range ops {
		if op.Op != "add" {
			t.Errorf("op = %q, want add", op.Op)
		}
	}
}

func TestBuildPatchNilPod(t *testing.T) {
	if patch := BuildPatch(nil, testDefaults()); patch != nil {
		t.Errorf("patch = %s, want nil", patch)
	}
}

func testDefaults() policy.Defaults {
	nonRoot := true
	noEscalation := false
	return policy.Defaults{
		PodSecurityContext: &kube.SecurityContext{
			RunAsNonRoot: &nonRoot,
			SeccompProfile: &kube.SeccompProfile{
				Type: "RuntimeDefault",
			},
		},
		ContainerSecurityContext: &kube.SecurityContext{
			AllowPrivilegeEscalation: &noEscalation,
			Capabilities: &kube.Capabilities{
				Drop: []string{"ALL"},
			},
		},
	}
}
