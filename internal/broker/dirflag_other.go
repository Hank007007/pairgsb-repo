//go:build !linux && !windows

package broker

// directoryFlag has no portable equivalent on this platform; the following
// operations still fail when the object is not a directory.
func directoryFlag() int {
	return 0
}
