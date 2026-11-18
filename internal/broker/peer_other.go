//go:build !linux

package broker

import (
	"fmt"
	"net"
	"runtime"
)

// kernelPeerUID is unavailable on this platform.
func kernelPeerUID(c *net.UnixConn) (int, error) {
	return 0, fmt.Errorf("broker: peer credential is not exposed on %s", runtime.GOOS)
}
