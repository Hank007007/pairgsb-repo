package manifest

import "strings"

// Select returns the target that applies to the device model.
func Select(m *Manifest, deviceModel string) (*Target, error) {
	if m == nil {
		return nil, ErrManifestInvalid
	}
	for i := range m.Targets {
		if strings.HasPrefix(m.Targets[i].Model, deviceModel) {
			return &m.Targets[i], nil
		}
	}
	return nil, ErrNoMatchingTarget
}
