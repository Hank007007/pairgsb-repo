// Package pathguard maps archive entry names onto locations inside a job root.
package pathguard

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pairgsb/unpackfence/internal/fault"
)

// CheckName validates an entry name before it is joined with a root.
func CheckName(name string) error {
	if name == "" || name == "." {
		return fmt.Errorf("%w: empty name", fault.ErrUnsafeName)
	}
	if strings.HasPrefix(name, "/") {
		return fmt.Errorf("%w: %q is absolute", fault.ErrUnsafeName, name)
	}
	return nil
}

// Resolve joins name onto root and reports the location the entry would use.
func Resolve(root, name string) (string, error) {
	if err := CheckName(name); err != nil {
		return "", err
	}
	root = filepath.Clean(root)
	target := filepath.Join(root, name)
	if !strings.HasPrefix(target, root) {
		return "", fmt.Errorf("%w: %q", fault.ErrOutsideRoot, name)
	}
	return target, nil
}
