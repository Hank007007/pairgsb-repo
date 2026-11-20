package config

// Merge combines the base document with an environment overlay. Both
// documents are flat maps whose values are strings or lists of strings.
//
// Scalar keys keep the base value when the base already has one, so an empty
// overlay entry never replaces a real value. List keys concatenate the base
// elements with the overlay elements.
func Merge(base, overlay map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(overlay))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range overlay {
		existing, ok := out[key]
		if !ok {
			out[key] = value
			continue
		}

		baseStr, baseIsString := existing.(string)
		overlayStr, overlayIsString := value.(string)
		if baseIsString && overlayIsString {
			if baseStr != "" {
				continue
			}
			out[key] = overlayStr
			continue
		}

		baseList, baseIsList := existing.([]string)
		overlayList, overlayIsList := value.([]string)
		if baseIsList && overlayIsList {
			joined := make([]string, 0, len(baseList)+len(overlayList))
			joined = append(joined, baseList...)
			joined = append(joined, overlayList...)
			out[key] = joined
			continue
		}

		out[key] = value
	}
	return out
}
