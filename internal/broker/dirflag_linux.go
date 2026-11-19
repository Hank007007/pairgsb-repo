//go:build linux

package broker

import "syscall"

// directoryFlag requests a directory open, so opening a swapped-in file fails.
func directoryFlag() int {
	return syscall.O_DIRECTORY
}
