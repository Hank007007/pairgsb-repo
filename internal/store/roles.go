package store

import (
	"encoding/json"
	"errors"
	"os"
	"sync"

	"verbgate/internal/authz"
)

// RoleRef names the role a binding grants.
type RoleRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// Subject is one grantee of a binding.
type Subject struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// Role is a Role or ClusterRole: a name plus a list of rules.
type Role struct {
	Name  string       `json:"name"`
	Rules []authz.Rule `json:"rules"`
}

// Binding is a RoleBinding or ClusterRoleBinding. Namespace is empty for
// cluster-scoped bindings.
type Binding struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace,omitempty"`
	RoleRef   RoleRef   `json:"roleRef"`
	Subjects  []Subject `json:"subjects"`
}

// Snapshot is the complete set of authorization data loaded from the control
// plane. Version changes whenever the snapshot is replaced.
type Snapshot struct {
	Version         string    `json:"version"`
	Roles           []Role    `json:"roles,omitempty"`
	ClusterRoles    []Role    `json:"clusterRoles,omitempty"`
	RoleBindings    []Binding `json:"roleBindings,omitempty"`
	ClusterBindings []Binding `json:"clusterBindings,omitempty"`
}

// Store holds the current snapshot and swaps it atomically on reload.
type Store struct {
	mu       sync.RWMutex
	snapshot *Snapshot
}

func New() *Store {
	return &Store{}
}

// Replace installs a new snapshot and returns its version.
func (s *Store) Replace(snapshot Snapshot) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	installed := snapshot
	s.snapshot = &installed
	return installed.Version
}

// Current returns the active snapshot. The result must be treated as read-only.
func (s *Store) Current() *Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.snapshot == nil {
		return &Snapshot{}
	}
	return s.snapshot
}

// Version returns the version of the active snapshot.
func (s *Store) Version() string {
	return s.Current().Version
}

// BindingsForNamespace returns the RoleBindings in namespace plus every
// ClusterRoleBinding. For a cluster-scoped request only ClusterRoleBindings
// apply.
func (s *Store) BindingsForNamespace(namespace string) []Binding {
	snapshot := s.Current()
	out := make([]Binding, 0, len(snapshot.RoleBindings)+len(snapshot.ClusterBindings))
	if namespace != "" {
		for _, binding := range snapshot.RoleBindings {
			if binding.Namespace == namespace {
				out = append(out, binding)
			}
		}
	}
	out = append(out, snapshot.ClusterBindings...)
	return out
}

// ResolveRole finds the role referenced by a binding.
func (s *Store) ResolveRole(binding Binding) (Role, bool) {
	snapshot := s.Current()
	switch binding.RoleRef.Kind {
	case "Role":
		for _, role := range snapshot.Roles {
			if role.Name == binding.RoleRef.Name {
				return role, true
			}
		}
	case "ClusterRole":
		for _, role := range snapshot.ClusterRoles {
			if role.Name == binding.RoleRef.Name {
				return role, true
			}
		}
	}
	return Role{}, false
}

// Candidates resolves the bindings that apply to namespace into the form the
// authz package evaluates. Bindings whose role cannot be found are skipped.
func (s *Store) Candidates(namespace string) []authz.Binding {
	bindings := s.BindingsForNamespace(namespace)
	out := make([]authz.Binding, 0, len(bindings))
	for _, binding := range bindings {
		role, ok := s.ResolveRole(binding)
		if !ok {
			continue
		}
		subjects := make([]authz.Subject, 0, len(binding.Subjects))
		for _, subject := range binding.Subjects {
			subjects = append(subjects, authz.Subject{Kind: subject.Kind, Name: subject.Name})
		}
		out = append(out, authz.Binding{
			Namespace: binding.Namespace,
			Subjects:  subjects,
			Rules:     role.Rules,
		})
	}
	return out
}

// LoadSnapshot reads a snapshot JSON file.
func LoadSnapshot(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}
	if snapshot.Version == "" {
		return nil, errors.New("snapshot: version is required")
	}
	return &snapshot, nil
}
