package kube

// Pod is the subset of the v1 Pod schema podgrated understands.
type Pod struct {
	Metadata ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec     PodSpec    `json:"spec" yaml:"spec"`
}

// PodSpec is the Pod-level part of a Pod's specification.
type PodSpec struct {
	Containers          []Container      `json:"containers,omitempty" yaml:"containers,omitempty"`
	InitContainers      []Container      `json:"initContainers,omitempty" yaml:"initContainers,omitempty"`
	EphemeralContainers []Container      `json:"ephemeralContainers,omitempty" yaml:"ephemeralContainers,omitempty"`
	Volumes             []Volume         `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	SecurityContext     *SecurityContext `json:"securityContext,omitempty" yaml:"securityContext,omitempty"`
	HostNetwork         bool             `json:"hostNetwork,omitempty" yaml:"hostNetwork,omitempty"`
	HostPID             bool             `json:"hostPID,omitempty" yaml:"hostPID,omitempty"`
	HostIPC             bool             `json:"hostIPC,omitempty" yaml:"hostIPC,omitempty"`
}

// Container is one entry of any of the three container lists.
type Container struct {
	Name            string           `json:"name" yaml:"name"`
	Image           string           `json:"image,omitempty" yaml:"image,omitempty"`
	SecurityContext *SecurityContext `json:"securityContext,omitempty" yaml:"securityContext,omitempty"`
	VolumeMounts    []VolumeMount    `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
}

// VolumeMount binds a volume into a container's filesystem.
type VolumeMount struct {
	Name      string `json:"name" yaml:"name"`
	MountPath string `json:"mountPath" yaml:"mountPath"`
	ReadOnly  bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
}

// Volume is one entry of spec.volumes.
type Volume struct {
	Name     string    `json:"name" yaml:"name"`
	HostPath *HostPath `json:"hostPath,omitempty" yaml:"hostPath,omitempty"`
}

// HostPath is a volume backed by a node path.
type HostPath struct {
	Path string `json:"path" yaml:"path"`
	Type string `json:"type,omitempty" yaml:"type,omitempty"`
}

// SecurityContext holds the security settings podgrated inspects and
// defaults. It appears both at the Pod level and on every container.
type SecurityContext struct {
	Privileged               *bool           `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	AllowPrivilegeEscalation *bool           `json:"allowPrivilegeEscalation,omitempty" yaml:"allowPrivilegeEscalation,omitempty"`
	RunAsUser                *int64          `json:"runAsUser,omitempty" yaml:"runAsUser,omitempty"`
	RunAsNonRoot             *bool           `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	Capabilities             *Capabilities   `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	SeccompProfile           *SeccompProfile `json:"seccompProfile,omitempty" yaml:"seccompProfile,omitempty"`
}

// Capabilities is the capability delta for a container.
type Capabilities struct {
	Add  []string `json:"add,omitempty" yaml:"add,omitempty"`
	Drop []string `json:"drop,omitempty" yaml:"drop,omitempty"`
}

// SeccompProfile selects the seccomp profile for a container or Pod.
type SeccompProfile struct {
	Type             string `json:"type" yaml:"type"`
	LocalhostProfile string `json:"localhostProfile,omitempty" yaml:"localhostProfile,omitempty"`
}
