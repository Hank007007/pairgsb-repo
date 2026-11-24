package mutate

import (
	"encoding/json"

	"podgrate/internal/kube"
	"podgrate/internal/policy"
)

// Op is one RFC 6902 JSONPatch operation.
type Op struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// BuildPatch computes the JSONPatch that writes the policy defaults into pod.
func BuildPatch(pod *kube.Pod, defaults policy.Defaults) []byte {
	if pod == nil {
		return nil
	}

	var ops []Op

	if defaults.PodSecurityContext != nil && pod.Spec.SecurityContext == nil {
		ops = append(ops, Op{
			Op:    "add",
			Path:  "/spec/securityContext",
			Value: *defaults.PodSecurityContext,
		})
	}

	if defaults.ContainerSecurityContext != nil {
		want := *defaults.ContainerSecurityContext

		for i := range pod.Spec.Containers {
			container := &pod.Spec.Containers[i]
			path := "/spec/containers/" + container.Name + "/securityContext"

			ops = append(ops, Op{Op: "add", Path: path, Value: want})

			if want.Capabilities != nil {
				for _, capability := range want.Capabilities.Drop {
					ops = append(ops, Op{
						Op:    "add",
						Path:  path + "/capabilities/drop/0",
						Value: capability,
					})
				}
			}
		}

		ops = append(ops, Op{
			Op:    "add",
			Path:  "/spec/initContainers/0/securityContext",
			Value: want,
		})
	}

	return encode(ops)
}

// encode renders the operations as a JSONPatch document.
func encode(ops []Op) []byte {
	if len(ops) == 0 {
		return nil
	}
	out, err := json.Marshal(ops)
	if err != nil {
		return nil
	}
	return out
}
