package runner

import (
	"sort"
	"strings"
)

// allowedEnvKeys is the environment allowlist from docs/argv-policy.md
// appendix A. The set must match the appendix exactly: matching is
// case-sensitive and by whole key, never by prefix.
var allowedEnvKeys = map[string]bool{
	"CI":             true,
	"CI_PIPELINE_ID": true,
	"CI_STEP_ID":     true,
	"HOME":           true,
	"LANG":           true,
	"LC_ALL":         true,
	"NO_COLOR":       true,
	"TZ":             true,
	"TMPDIR":         true,
}

// mergeEnv filters the runner's environment and the entries declared by the
// plugin through the allowlist and merges them into the final environment.
//
// A declared entry overrides a base entry with the same key, so every key
// appears exactly once (policy §3.3). The result is sorted by key so that
// repeated builds are byte-for-byte identical (§6.2). Keys rejected by the
// allowlist are reported in dropped, sorted and deduplicated (§3.1).
func mergeEnv(baseEnv []string, declared []string) (env []string, dropped []string) {
	merged := make(map[string]string, len(baseEnv)+len(declared))
	droppedSet := map[string]bool{}

	for _, entry := range baseEnv {
		key, value, _ := strings.Cut(entry, "=")
		if !allowedEnvKeys[key] {
			droppedSet[key] = true
			continue
		}
		merged[key] = value
	}
	for _, entry := range declared {
		key, value, _ := strings.Cut(entry, "=")
		if !allowedEnvKeys[key] {
			droppedSet[key] = true
			continue
		}
		merged[key] = value
	}

	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	env = make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+merged[key])
	}

	for key := range droppedSet {
		dropped = append(dropped, key)
	}
	sort.Strings(dropped)
	return env, dropped
}
