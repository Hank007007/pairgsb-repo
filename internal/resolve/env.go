package resolve

import (
	"context"
	"fmt"
	"os"
)

// EnvProvider reads secrets from process environment variables.
type EnvProvider struct {
	// Lookup defaults to os.LookupEnv.
	Lookup func(string) (string, bool)
}

// NewEnvProvider returns a provider backed by the process environment.
func NewEnvProvider() *EnvProvider {
	return &EnvProvider{Lookup: os.LookupEnv}
}

// Scheme implements Provider.
func (e *EnvProvider) Scheme() string { return SchemeEnv }

// Resolve returns the value of the environment variable named by ref.Path.
func (e *EnvProvider) Resolve(ctx context.Context, scope string, ref Ref) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	lookup := e.Lookup
	if lookup == nil {
		lookup = os.LookupEnv
	}
	value, ok := lookup(ref.Path)
	if !ok {
		return "", fmt.Errorf("env: variable %q is not set", ref.Path)
	}
	return value, nil
}
