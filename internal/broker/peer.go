package broker

import (
	"errors"
	"net"
)

var errNoPeerCredential = errors.New("broker: peer credential is not available")

// unixConnExposer is implemented by connections that wrap an underlying
// *net.UnixConn, such as the buffered Conn produced by the listener.
type unixConnExposer interface {
	Unix() *net.UnixConn
}

// peerIdentity returns the uid the broker attributes to the peer of c. The
// only acceptable evidence of a caller's identity is the credential the
// kernel attaches to the connected socket. Environment variables, request
// fields and command line arguments are never consulted: when the kernel
// credential cannot be read the connection is refused.
func peerIdentity(c net.Conn) (int, error) {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		if exposer, isWrapped := c.(unixConnExposer); isWrapped {
			uc = exposer.Unix()
		}
	}
	if uc == nil {
		return 0, errNoPeerCredential
	}
	return kernelPeerUID(uc)
}
