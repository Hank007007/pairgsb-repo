// Package config loads and validates the hopfence configuration file.
//
// The configuration is operator-supplied and is never derived from client
// input. Both the upstream list and the routing table are fixed at start-up.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"net/url"
	"os"
	"strings"
)

// Defaults applied when the corresponding configuration field is omitted.
const (
	DefaultListen         = "127.0.0.1:8080"
	DefaultTimeoutSeconds = 30
)

// Route describes a single path-prefix route.
//
// Allow and Deny hold source addresses as CIDR prefixes or plain IP
// addresses. DenyPaths holds path prefixes that the route refuses to serve.
type Route struct {
	Name      string   `json:"name"`
	Prefix    string   `json:"prefix"`
	Upstream  string   `json:"upstream"`
	Allow     []string `json:"allow,omitempty"`
	Deny      []string `json:"deny,omitempty"`
	DenyPaths []string `json:"deny_paths,omitempty"`
}

// Config is the top-level configuration document.
type Config struct {
	Listen         string  `json:"listen"`
	AccessLog      string  `json:"access_log,omitempty"`
	TimeoutSeconds int     `json:"timeout_seconds,omitempty"`
	Routes         []Route `json:"routes"`
}

// Load reads, parses, normalizes, and validates the configuration at path.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	return Parse(data)
}

// Parse decodes, normalizes, and validates a configuration document.
func Parse(data []byte) (Config, error) {
	var cfg Config
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Config{}, errors.New("decode config: trailing data after document")
	}
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Listen == "" {
		c.Listen = DefaultListen
	}
	if c.TimeoutSeconds == 0 {
		c.TimeoutSeconds = DefaultTimeoutSeconds
	}
}

// Validate checks every configuration invariant. It returns the first
// violation it encounters.
func (c *Config) Validate() error {
	if _, _, err := net.SplitHostPort(c.Listen); err != nil {
		return fmt.Errorf("listen %q: %w", c.Listen, err)
	}
	if c.TimeoutSeconds < 1 {
		return fmt.Errorf("timeout_seconds must be positive, got %d", c.TimeoutSeconds)
	}
	if len(c.Routes) == 0 {
		return errors.New("at least one route is required")
	}
	seen := make(map[string]struct{}, len(c.Routes))
	for i := range c.Routes {
		if err := c.Routes[i].Validate(); err != nil {
			return fmt.Errorf("routes[%d]: %w", i, err)
		}
		name := c.Routes[i].Name
		if _, dup := seen[name]; dup {
			return fmt.Errorf("routes[%d]: duplicate route name %q", i, name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

// Validate checks the invariants of a single route.
func (r *Route) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("name is required")
	}
	if !strings.HasPrefix(r.Prefix, "/") {
		return fmt.Errorf("prefix %q must start with /", r.Prefix)
	}
	if strings.ContainsAny(r.Prefix, "?#") {
		return fmt.Errorf("prefix %q must not contain a query or fragment", r.Prefix)
	}
	u, err := url.Parse(r.Upstream)
	if err != nil {
		return fmt.Errorf("upstream %q: %w", r.Upstream, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("upstream %q: scheme must be http or https", r.Upstream)
	}
	if u.Host == "" {
		return fmt.Errorf("upstream %q: host is required", r.Upstream)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("upstream %q: query and fragment are not allowed", r.Upstream)
	}
	if _, err := ParseCIDRs(r.Allow); err != nil {
		return fmt.Errorf("allow: %w", err)
	}
	if _, err := ParseCIDRs(r.Deny); err != nil {
		return fmt.Errorf("deny: %w", err)
	}
	for _, p := range r.DenyPaths {
		if !strings.HasPrefix(p, "/") {
			return fmt.Errorf("deny_paths entry %q must start with /", p)
		}
		if strings.ContainsAny(p, "?#") {
			return fmt.Errorf("deny_paths entry %q must not contain a query or fragment", p)
		}
	}
	return nil
}

// ParseCIDRs converts a list of CIDR prefixes or plain IP addresses into
// netip prefixes. A plain address is treated as a single-host prefix.
func ParseCIDRs(entries []string) ([]netip.Prefix, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	out := make([]netip.Prefix, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return nil, errors.New("empty address entry")
		}
		if strings.Contains(entry, "/") {
			prefix, err := netip.ParsePrefix(entry)
			if err != nil {
				return nil, fmt.Errorf("%q: %w", entry, err)
			}
			out = append(out, prefix.Masked())
			continue
		}
		addr, err := netip.ParseAddr(entry)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", entry, err)
		}
		addr = addr.Unmap()
		out = append(out, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return out, nil
}
