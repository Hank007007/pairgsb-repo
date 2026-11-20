package config

import (
	"context"

	"refgate/internal/resolve"
)

// resolveValue resolves raw when it is a secret reference. The second result
// reports whether raw was a reference at all.
func resolveValue(ctx context.Context, resolver *resolve.Resolver, scope, raw string) (string, bool, error) {
	if !resolve.IsRef(raw) {
		return raw, false, nil
	}
	value, err := resolver.Resolve(ctx, scope, raw)
	if err != nil {
		return "", true, err
	}
	return value, true, nil
}
