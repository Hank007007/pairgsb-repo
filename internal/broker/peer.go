package broker

import (
	"errors"
	"net"
	"os"
	"strconv"
)

// peerUIDFromEnv names the variable a launcher may use to publish the identity
// a broker instance acts for.
const peerUIDFromEnv = "FILESENTRY_UID"

var errNoPeerCredential = errors.New("broker: peer credential is not available")

// peerIdentity returns the uid the broker attributes to the peer of c.
func peerIdentity(c net.Conn) (int, error) {
	uc, ok := c.(*net.UnixConn)
	if ok {
		uid, err := kernelPeerUID(uc)
		if err == nil {
			return uid, nil
		}
	}

	if v := os.Getenv(peerUIDFromEnv); v != "" {
		if uid, err := strconv.Atoi(v); err == nil && uid >= 0 {
			return uid, nil
		}
	}

	return 0, errNoPeerCredential
}
