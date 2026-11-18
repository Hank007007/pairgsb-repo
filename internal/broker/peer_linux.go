//go:build linux

package broker

import (
	"fmt"
	"net"
	"syscall"
)

// kernelPeerUID reads the credential the kernel attached to the connection.
func kernelPeerUID(c *net.UnixConn) (int, error) {
	raw, err := c.SyscallConn()
	if err != nil {
		return 0, err
	}

	var cred *syscall.Ucred
	var credErr error
	if err := raw.Control(func(fd uintptr) {
		cred, credErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil {
		return 0, err
	}
	if credErr != nil {
		return 0, credErr
	}
	if cred == nil {
		return 0, fmt.Errorf("broker: empty peer credential")
	}
	return int(cred.Uid), nil
}
