package policy

// MatchLabels reports whether every label in selector is present with the
// same value in labels. An empty selector matches everything.
func MatchLabels(labels, selector map[string]string) bool {
	for key, want := range selector {
		got, ok := labels[key]
		if !ok || got != want {
			return false
		}
	}
	return true
}
