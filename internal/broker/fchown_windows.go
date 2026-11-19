//go:build windows

package broker

import "os"

// fchown falls back to the portable Chown call on platforms without fchown.
// The handle names the resolved object that already passed containment.
func fchown(f *os.File, uid, gid int) error {
	return f.Chown(uid, gid)
}
