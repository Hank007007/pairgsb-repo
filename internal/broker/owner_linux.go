//go:build linux

package broker

import (
	"os"
	"syscall"
)

// ownerOf reports the uid and gid recorded for fi.
func ownerOf(fi os.FileInfo) (int, int, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return int(st.Uid), int(st.Gid), true
}
