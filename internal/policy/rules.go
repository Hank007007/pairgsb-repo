package policy

import (
	"fmt"
	"strings"

	"podgrate/internal/kube"
)

// Rule identifiers surfaced in admission responses and logs.
const (
	RuleHostPath      = "podgrate/hostpath"
	RuleHostNamespace = "podgrate/hostnamespace"
	RulePrivilege     = "podgrate/privilege"
	RuleCapabilities  = "podgrate/capabilities"
	RuleRegistry      = "podgrate/registry"
	RulePolicy        = "podgrate/policy"
)

// Violation is one rule that fired against a Pod.
type Violation struct {
	Rule    string
	Message string

	// Image and Path carry the manifest values the rule fired on. They are
	// kept apart from Message so a caller can decide how much of them to
	// surface.
	Image string
	Path  string
}

// Result is the outcome of evaluating a Pod against a policy.
type Result struct {
	Allowed    bool
	Violations []Violation
}

// Message renders the violations as a single line.
func (r Result) Message() string {
	parts := make([]string, 0, len(r.Violations))
	for _, v := range r.Violations {
		parts = append(parts, v.Rule+": "+v.Message)
	}
	return strings.Join(parts, "; ")
}

// Evaluate applies the policy rules to pod.
func Evaluate(p *Policy, pod *kube.Pod) Result {
	var res Result

	if p == nil {
		res.Violations = append(res.Violations, Violation{
			Rule:    RulePolicy,
			Message: "no policy is in effect",
		})
		return res
	}
	if pod == nil {
		res.Violations = append(res.Violations, Violation{
			Rule:    RulePolicy,
			Message: "the review carries no pod",
		})
		return res
	}

	spec := &pod.Spec

	if spec.HostNetwork {
		res.Violations = append(res.Violations, Violation{
			Rule:    RuleHostNamespace,
			Message: "the pod uses the host network namespace",
		})
	}

	if sc := spec.SecurityContext; sc != nil {
		if sc.Privileged != nil && *sc.Privileged {
			res.Violations = append(res.Violations, Violation{
				Rule:    RulePrivilege,
				Message: "the pod security context sets privileged",
			})
		}
		if sc.AllowPrivilegeEscalation != nil && *sc.AllowPrivilegeEscalation {
			res.Violations = append(res.Violations, Violation{
				Rule:    RulePrivilege,
				Message: "the pod security context allows privilege escalation",
			})
		}
		if sc.RunAsUser != nil && *sc.RunAsUser == 0 {
			res.Violations = append(res.Violations, Violation{
				Rule:    RulePrivilege,
				Message: "the pod security context runs as uid 0",
			})
		}
	}

	for i := range spec.Containers {
		container := &spec.Containers[i]

		if !imageAllowed(container.Image, p.AllowedRegistries) {
			res.Violations = append(res.Violations, Violation{
				Rule:    RuleRegistry,
				Message: fmt.Sprintf("container %q pulls from outside the allowed registries", container.Name),
				Image:   container.Image,
			})
		}

		for _, capability := range addedCapabilities(container) {
			if !capabilityAllowed(capability, p.AllowedCapabilities) {
				res.Violations = append(res.Violations, Violation{
					Rule:    RuleCapabilities,
					Message: fmt.Sprintf("container %q adds capability %q", container.Name, capability),
				})
			}
		}
	}

	for i := range spec.Volumes {
		volume := &spec.Volumes[i]
		if volume.HostPath == nil {
			continue
		}
		if !hostPathAllowed(volume.HostPath.Path, p.AllowedHostPaths) {
			res.Violations = append(res.Violations, Violation{
				Rule:    RuleHostPath,
				Message: fmt.Sprintf("volume %q mounts a host path", volume.Name),
				Path:    volume.HostPath.Path,
			})
		}
	}

	res.Allowed = len(res.Violations) == 0
	return res
}

func addedCapabilities(container *kube.Container) []string {
	if container.SecurityContext == nil || container.SecurityContext.Capabilities == nil {
		return nil
	}
	return container.SecurityContext.Capabilities.Add
}

// capabilityAllowed reports whether an added capability may be granted.
func capabilityAllowed(capability string, allowed []string) bool {
	for _, name := range allowed {
		if capability == name {
			return true
		}
	}
	return capability != "ALL"
}

// imageAllowed reports whether the image is served by a registry the policy
// names.
func imageAllowed(image string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, registry := range allowed {
		if strings.Contains(image, registry+"/") {
			return true
		}
	}
	return false
}

// hostPathAllowed reports whether path lies in one of the trees the policy
// allows.
func hostPathAllowed(path string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, tree := range allowed {
		if strings.HasPrefix(path, tree) {
			return true
		}
	}
	return false
}
