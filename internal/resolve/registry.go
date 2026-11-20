// Package resolve turns secret references into values.
//
// A reference has the form secret://<scheme>/<path>. Providers register
// themselves with a Registry under a scheme name; the Resolver adds a
// TTL cache in front of the registry lookup.
package resolve

import (
	"context"
	"fmt"
	"strings"
)

// Prefix marks a string as a secret reference.
const Prefix = "secret://"

// Scheme names understood by the registry.
const (
	SchemeKV   = "kv"
	SchemeEnv  = "env"
	SchemeFile = "file"
)

// Ref is a parsed secret reference.
type Ref struct {
	Scheme string
	Path   string
}

// String renders the reference back to its canonical form.
func (r Ref) String() string {
	return Prefix + r.Scheme + "/" + r.Path
}

// ParseRef parses raw into a Ref. The scheme is not validated here; dispatch
// happens in Registry.Resolve.
func ParseRef(raw string) (Ref, error) {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, Prefix) {
		return Ref{}, fmt.Errorf("resolve: %q is not a secret reference", raw)
	}
	rest := s[len(Prefix):]
	scheme, path, ok := strings.Cut(rest, "/")
	if !ok || scheme == "" || path == "" {
		return Ref{}, fmt.Errorf("resolve: malformed secret reference %q", raw)
	}
	return Ref{Scheme: scheme, Path: path}, nil
}

// IsRef reports whether s looks like a secret reference.
func IsRef(s string) bool {
	return strings.HasPrefix(strings.TrimSpace(s), Prefix)
}

// Provider resolves references for one scheme. scope is the environment the
// configuration is being loaded for and may be ignored by providers that do
// not partition their data by environment.
type Provider interface {
	Scheme() string
	Resolve(ctx context.Context, scope string, ref Ref) (string, error)
}

// Registry dispatches references to the provider registered for their scheme.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

// Register adds p to the registry. A provider registered for an existing
// scheme replaces the previous one.
func (r *Registry) Register(p Provider) {
	if p == nil {
		return
	}
	r.providers[p.Scheme()] = p
}

// HasProvider reports whether a provider is registered for scheme.
func (r *Registry) HasProvider(scheme string) bool {
	_, ok := r.providers[scheme]
	return ok
}

// Schemes lists the registered scheme names in sorted order.
func (r *Registry) Schemes() []string {
	out := make([]string, 0, len(r.providers))
	for name := range r.providers {
		out = append(out, name)
	}
	sortStrings(out)
	return out
}

// Resolve looks up the provider for ref.Scheme and asks it for the value.
func (r *Registry) Resolve(ctx context.Context, scope string, ref Ref) (string, error) {
	var p Provider
	switch ref.Scheme {
	case SchemeKV:
		p = r.providers[SchemeKV]
	case SchemeEnv:
		p = r.providers[SchemeEnv]
	case SchemeFile:
		p = r.providers[SchemeFile]
	default:
		// References that do not name a dedicated provider are served by the
		// file provider.
		p = r.providers[SchemeFile]
	}
	if p == nil {
		return "", fmt.Errorf("resolve: no provider for scheme %q", ref.Scheme)
	}
	return p.Resolve(ctx, scope, ref)
}

// Resolver combines a Registry with a cache.
type Resolver struct {
	registry *Registry
	cache    *Cache
}

// NewResolver wires a registry to a cache. A nil cache disables caching.
func NewResolver(registry *Registry, cache *Cache) *Resolver {
	if registry == nil {
		registry = NewRegistry()
	}
	return &Resolver{registry: registry, cache: cache}
}

// Resolve resolves raw in the given environment scope.
func (r *Resolver) Resolve(ctx context.Context, scope, raw string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ref, err := ParseRef(raw)
	if err != nil {
		return "", err
	}
	if r.cache == nil {
		return r.registry.Resolve(ctx, scope, ref)
	}
	return r.cache.Get(raw, func() (string, error) {
		return r.registry.Resolve(ctx, scope, ref)
	})
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
