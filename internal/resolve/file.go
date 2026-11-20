package resolve

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileProvider reads secrets from files below a fixed set of directories.
type FileProvider struct {
	roots []string
}

// NewFileProvider returns a provider restricted to roots. Paths that resolve
// outside every root are rejected.
func NewFileProvider(roots ...string) *FileProvider {
	clean := make([]string, 0, len(roots))
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		clean = append(clean, root)
	}
	return &FileProvider{roots: clean}
}

// Roots returns the whitelisted directories.
func (f *FileProvider) Roots() []string {
	out := make([]string, len(f.roots))
	copy(out, f.roots)
	return out
}

// Scheme implements Provider.
func (f *FileProvider) Scheme() string { return SchemeFile }

// Resolve reads ref.Path relative to the first whitelisted directory that
// contains it. Surrounding newlines are stripped from the file contents.
func (f *FileProvider) Resolve(ctx context.Context, scope string, ref Ref) (string, error) {
	if len(f.roots) == 0 {
		return "", errors.New("file: no whitelisted directories configured")
	}
	rel := filepath.Clean(filepath.FromSlash(strings.TrimSpace(ref.Path)))
	if rel == "." || rel == ".." || filepath.IsAbs(rel) ||
		strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("file: %q is outside the whitelisted directories", ref.Path)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	for _, root := range f.roots {
		full := filepath.Join(root, rel)
		if !within(root, full) {
			continue
		}
		data, err := os.ReadFile(full)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", fmt.Errorf("file: read %q: %w", ref.Path, err)
		}
		return strings.TrimRight(string(data), "\r\n"), nil
	}
	return "", fmt.Errorf("file: %q not found in the whitelisted directories", ref.Path)
}

// within reports whether full is inside root after resolving symlinks.
func within(root, full string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = resolved
	}
	if resolved, err := filepath.EvalSymlinks(absFull); err == nil {
		absFull = resolved
	}
	rel, err := filepath.Rel(absRoot, absFull)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
