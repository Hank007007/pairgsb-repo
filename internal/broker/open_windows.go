//go:build windows

package broker

import "os"

// openResolved opens the object whose fully resolved path already passed
// containment. Windows exposes neither openat nor O_NOFOLLOW, so the portable
// build opens the resolved path directly; containment itself was established
// after resolving every symlink.
func openResolved(resolved string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(resolved, flag, perm)
}

// openDirectory opens an existing directory for a list request.
func openDirectory(resolved string) (*os.File, error) {
	return os.Open(resolved)
}

func classifyOpenError(err error) error {
	return err
}

func isSymlinkLoop(err error) bool {
	return false
}
