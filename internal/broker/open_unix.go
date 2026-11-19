//go:build !windows

package broker

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// openResolved opens the already-validated object. The parent directory is
// opened with O_NOFOLLOW so a symlink swapped in after the containment check
// cannot redirect the open, and the final component is opened relative to that
// directory, keeping the check and the operation on the same object.
func openResolved(resolved string, flag int, perm os.FileMode) (*os.File, error) {
	dirName, name := filepath.Split(filepath.FromSlash(resolved))
	if dirName == "" || name == "" {
		return os.OpenFile(filepath.FromSlash(resolved), flag, perm)
	}

	dir, err := os.OpenFile(dirName, os.O_RDONLY|syscall.O_NOFOLLOW|directoryFlag(), 0)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	return openAt(dir, name, flag, perm)
}

// openDirectory opens an existing directory for a list request, refusing to
// follow a symlink that replaced the validated object.
func openDirectory(resolved string) (*os.File, error) {
	dirName, name := filepath.Split(filepath.FromSlash(resolved))
	if dirName == "" || name == "" {
		return os.Open(filepath.FromSlash(resolved))
	}

	parent, err := os.OpenFile(dirName, os.O_RDONLY|syscall.O_NOFOLLOW|directoryFlag(), 0)
	if err != nil {
		return nil, err
	}
	defer parent.Close()

	return openAt(parent, name, os.O_RDONLY|syscall.O_NOFOLLOW|directoryFlag(), 0)
}

func isSymlinkLoop(err error) bool {
	return errors.Is(err, syscall.ELOOP)
}
