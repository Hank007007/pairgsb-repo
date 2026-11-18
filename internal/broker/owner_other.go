//go:build !linux

package broker

import "os"

// ownerOf reports the uid and gid recorded for fi. The platform does not
// expose them through a portable file info value.
func ownerOf(fi os.FileInfo) (int, int, bool) {
	return 0, 0, false
}
