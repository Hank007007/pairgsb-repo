package runner

// mergeEnv combines the runner's environment with the entries declared by the
// plugin and returns the result together with the keys that were not passed
// through.
//
// baseEnv is copied first; declared entries follow in declaration order.
func mergeEnv(baseEnv []string, declared []string) (env []string, dropped []string) {
	env = make([]string, 0, len(baseEnv)+len(declared))
	env = append(env, baseEnv...)
	env = append(env, declared...)

	return env, dropped
}
