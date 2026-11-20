package config

import (
	"sort"
)

// Placeholder replaces every value that must not appear in a dump.
const Placeholder = "<redacted>"

// Config is the effective configuration for one environment.
type Config struct {
	scope string
	raw   map[string]any
	meta  map[string]meta
	order []string
}

type meta struct {
	// secret is true when the effective value was produced from a reference.
	secret bool
	// ref is the reference text a scalar value was resolved from, if any.
	ref string
}

func newConfig(scope string) *Config {
	return &Config{
		scope: scope,
		raw:   make(map[string]any),
		meta:  make(map[string]meta),
	}
}

func (c *Config) put(key string, value any, m meta) {
	if _, exists := c.raw[key]; !exists {
		c.order = append(c.order, key)
	}
	c.raw[key] = value
	c.meta[key] = m
}

// Scope reports the environment the configuration was loaded for.
func (c *Config) Scope() string { return c.scope }

// Keys lists every key in sorted order.
func (c *Config) Keys() []string {
	out := make([]string, len(c.order))
	copy(out, c.order)
	sort.Strings(out)
	return out
}

// Get returns the scalar value stored under key.
func (c *Config) Get(key string) (string, bool) {
	v, ok := c.raw[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// List returns the list value stored under key.
func (c *Config) List(key string) ([]string, bool) {
	v, ok := c.raw[key]
	if !ok {
		return nil, false
	}
	list, ok := v.([]string)
	return list, ok
}

// IsSecret reports whether the effective value under key came from a
// reference.
func (c *Config) IsSecret(key string) bool {
	m, ok := c.meta[key]
	return ok && m.secret
}

// Ref reports the reference a scalar key was resolved from.
func (c *Config) Ref(key string) (string, bool) {
	m, ok := c.meta[key]
	if !ok || m.ref == "" {
		return "", false
	}
	return m.ref, true
}
