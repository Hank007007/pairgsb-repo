package broker

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"filesentry/internal/acl"
)

// containedTarget is the result of establishing that a request target stays
// inside the root it was authorised against. File is an already-open handle to
// the object that passed the check, so the broker never re-resolves the path
// and cannot be swapped onto a different object between check and use.
type containedTarget struct {
	file     *os.File
	resolved string
}

// resolveFile follows every intermediate symlink of target, verifies that the
// final location lies inside root, and opens that same object with flag. A
// symlink whose target leaves root is reported as path_escape even if the
// outside object would be readable.
func resolveFile(target, root string, flag int, perm os.FileMode) (*containedTarget, error) {
	target = acl.Clean(filepath.ToSlash(target))
	root = acl.Clean(filepath.ToSlash(root))
	if !containsPath(root, target) {
		return nil, errPathEscape
	}

	resolved, err := resolveContained(target, root)
	if err != nil {
		return nil, err
	}

	file, err := openResolved(resolved, flag, perm)
	if err != nil {
		return nil, classifyOpenError(err)
	}
	return &containedTarget{file: file, resolved: resolved}, nil
}

// resolveDirectory is resolveFile for directories, returning an open handle to
// the directory that passed containment.
func resolveDirectory(target, root string) (*containedTarget, error) {
	target = acl.Clean(filepath.ToSlash(target))
	root = acl.Clean(filepath.ToSlash(root))
	if !containsPath(root, target) {
		return nil, errPathEscape
	}

	resolved, err := resolveContained(target, root)
	if err != nil {
		return nil, err
	}

	dir, err := openDirectory(resolved)
	if err != nil {
		return nil, err
	}
	return &containedTarget{file: dir, resolved: resolved}, nil
}

// resolveContained evaluates all symlinks on the way to target and confirms
// the resulting location is still inside root.
func resolveContained(target, root string) (string, error) {
	resolvedRoot, err := filepath.EvalSymlinks(filepath.FromSlash(root))
	if err != nil {
		return "", err
	}

	resolvedTarget, err := filepath.EvalSymlinks(filepath.FromSlash(target))
	if err != nil {
		return "", classifyResolveError(err)
	}

	slashRoot := acl.Clean(resolvedRoot)
	slashTarget := acl.Clean(resolvedTarget)
	if !containsPath(slashRoot, slashTarget) {
		return "", errPathEscape
	}
	return slashTarget, nil
}

// containsPath reports whether p is root itself or lies strictly below it.
// The comparison is segment based: /srv/acme does not contain /srv/acme-evil.
func containsPath(root, p string) bool {
	root = acl.Clean(filepath.ToSlash(root))
	p = acl.Clean(filepath.ToSlash(p))
	if root == "/" {
		return strings.HasPrefix(p, "/")
	}
	return p == root || strings.HasPrefix(p, root+"/")
}

// classifyResolveError turns a symlink loop or other resolution failure that
// leaves containment unproven into path_escape; ordinary missing objects stay
// not_found.
func classifyResolveError(err error) error {
	if isSymlinkLoop(err) {
		return errPathEscape
	}
	if errors.Is(err, os.ErrNotExist) {
		return err
	}
	return err
}
