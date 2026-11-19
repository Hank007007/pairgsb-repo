//go:build !windows

package broker

import (
	"os"
	"syscall"
)

// fchown changes ownership on the already-validated open handle, so the
// operation applies to the same object that passed containment.
func fchown(f *os.File, uid, gid int) error {
	return syscall.Fchown(int(f.Fd()), uid, gid)
}
