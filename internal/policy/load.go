package policy

import (
	"os"

	"gopkg.in/yaml.v3"

	"podgrate/internal/kube"
)

// Policy is the in-memory form of the policy file. Field names follow the
// YAML document described in docs/policy-source.md.
type Policy struct {
	Version             int              `yaml:"version"`
	AllowedHostPaths    []string         `yaml:"allowedHostPaths"`
	AllowedRegistries   []string         `yaml:"allowedRegistries"`
	AllowedCapabilities []string         `yaml:"allowedCapabilities"`
	Defaults            Defaults         `yaml:"defaults"`
	Namespaces          []NamespaceEntry `yaml:"namespaces"`
	Default             *Overrides       `yaml:"default"`
}

// Defaults are the security settings the patch fills in for absent fields.
type Defaults struct {
	PodSecurityContext       *kube.SecurityContext `yaml:"podSecurityContext"`
	ContainerSecurityContext *kube.SecurityContext `yaml:"containerSecurityContext"`
}

// NamespaceEntry is the policy that applies to a set of namespaces, selected
// by the labels in Labels.
type NamespaceEntry struct {
	Name      string            `yaml:"name"`
	Labels    map[string]string `yaml:"labels"`
	Enabled   *bool             `yaml:"enabled"`
	Overrides Overrides         `yaml:"overrides"`
}

// Overrides replaces selected top-level values. A nil slice leaves the
// top-level value in place.
type Overrides struct {
	AllowedHostPaths    []string  `yaml:"allowedHostPaths"`
	AllowedRegistries   []string  `yaml:"allowedRegistries"`
	AllowedCapabilities []string  `yaml:"allowedCapabilities"`
	Defaults            *Defaults `yaml:"defaults"`
}

// LoadPolicy reads and parses the policy file at path. The file is read afresh
// for every review, so an edit is picked up on the next request.
func LoadPolicy(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return &Policy{}, nil
	}

	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return &Policy{}, err
	}
	return &p, nil
}
