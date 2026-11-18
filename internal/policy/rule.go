// Package policy loads call tables and decides whether a peer service may
// invoke an RPC method.
package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Effects a rule may carry.
const (
	EffectPermit Effect = "permit"
	EffectDeny   Effect = "deny"
)

// Errors returned by the loader and evaluator.
var (
	ErrBadRule     = errors.New("policy: malformed rule")
	ErrBadDocument = errors.New("policy: malformed policy document")
	ErrNoTrust     = errors.New("policy: trust configuration is missing")
)

// Effect is a rule's outcome.
type Effect string

// Rule is one line of a call table.
type Rule struct {
	// ID names the rule for audit purposes.
	ID string `json:"id"`
	// From is the peer service identity the rule applies to.
	From string `json:"from"`
	// To is the RPC method the rule applies to.
	To string `json:"to"`
	// Effect is either "permit" or "deny".
	Effect Effect `json:"effect"`
}

// Document is the on-disk shape of a call table file.
type Document struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Version     int    `json:"version"`
	Rules       []Rule `json:"rules"`
}

// Trust is the content of policies/trust.json.
type Trust struct {
	TrustDomain             string `json:"trust_domain"`
	AllowedClockSkewSeconds int    `json:"allowed_clock_skew_seconds,omitempty"`
}

// LoadTrust reads the local trust configuration.
func LoadTrust(path string) (*Trust, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoTrust
		}
		return nil, fmt.Errorf("policy: read trust: %w", err)
	}
	var t Trust
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("policy: parse trust: %w", err)
	}
	if strings.TrimSpace(t.TrustDomain) == "" {
		return nil, fmt.Errorf("%w: trust_domain is empty", ErrNoTrust)
	}
	return &t, nil
}

// parseDocument decodes and validates one call table.
func parseDocument(raw []byte, source string) (*Document, error) {
	var doc Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrBadDocument, source, err)
	}
	if doc.Name == "" {
		return nil, fmt.Errorf("%w: %s: name is empty", ErrBadDocument, source)
	}
	for i, r := range doc.Rules {
		if err := validateRule(r); err != nil {
			return nil, fmt.Errorf("%w: %s: rule %d: %v", ErrBadRule, source, i, err)
		}
	}
	return &doc, nil
}

func validateRule(r Rule) error {
	if r.From == "" {
		return errors.New("from is empty")
	}
	if r.To == "" {
		return errors.New("to is empty")
	}
	switch r.Effect {
	case EffectPermit, EffectDeny:
	default:
		return fmt.Errorf("unknown effect %q", r.Effect)
	}
	if strings.Contains(r.From, "//") || strings.HasPrefix(r.From, "/") {
		return fmt.Errorf("from %q must be a bare service identity", r.From)
	}
	if !strings.HasPrefix(r.To, "/") {
		return fmt.Errorf("to %q must be an absolute method name", r.To)
	}
	return nil
}

// policyFiles lists the call table files under dir, skipping the trust
// configuration.
func policyFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		if strings.EqualFold(e.Name(), "trust.json") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	return out, nil
}
