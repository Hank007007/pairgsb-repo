//go:build !windows

package broker

import (
	"errors"
	"os"
	"syscall"
)

// openAt opens name relative to the directory handle dir without following a
// symlink in the final component.
func openAt(dir *os.File, name string, flag int, perm os.FileMode) (*os.File, error) {
	fd, err := syscall.Openat(int(dir.Fd()), name, flag|syscall.O_NOFOLLOW, uint32(perm.Perm()))
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}

// classifyOpenError maps a refusal to follow a swapped symlink to path_escape.
func classifyOpenError(err error) error {
	if errors.Is(err, syscall.ELOOP) {
		return errPathEscape
	}
	return err
}
