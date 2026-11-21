package manifest

// Select returns the target that applies to the device model.
func Select(m *Manifest, deviceModel string) (*Target, error) {
	if m == nil {
		return nil, ErrManifestInvalid
	}
	var match *Target
	for i := range m.Targets {
		if m.Targets[i].Model != deviceModel {
			continue
		}
		if match != nil {
			return nil, ErrManifestInvalid
		}
		match = &m.Targets[i]
	}
	if match == nil {
		return nil, ErrNoMatchingTarget
	}
	return match, nil
}
