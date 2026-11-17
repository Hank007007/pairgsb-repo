// Package bundle is the plaintext carried inside a sealed envelope: an ordered
// set of named secrets.
package bundle

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Secret is one named credential.
type Secret struct {
	Name  string `json:"name"`
	Value []byte `json:"value"`
}

// Encode serializes the secret set in a stable order.
func Encode(secrets []Secret) ([]byte, error) {
	sorted := make([]Secret, len(secrets))
	copy(sorted, secrets)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	return json.Marshal(sorted)
}

// Decode parses a secret set, rejecting names that could escape the secrets
// directory.
func Decode(data []byte) ([]Secret, error) {
	var secrets []Secret
	if err := json.Unmarshal(data, &secrets); err != nil {
		return nil, fmt.Errorf("bundle: decode: %w", err)
	}
	for _, sec := range secrets {
		if err := ValidateName(sec.Name); err != nil {
			return nil, err
		}
	}
	return secrets, nil
}

// ValidateName rejects empty names, path separators, and relative traversal.
func ValidateName(name string) error {
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("bundle: invalid secret name %q", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("bundle: invalid secret name %q", name)
	}
	return nil
}
