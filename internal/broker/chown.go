package broker

import (
	"fmt"
	"os"

	"filesentry/internal/acl"
	"filesentry/internal/wire"
)

// Chown applies a chown request made by uid.
func (h *Handler) Chown(uid int, req *wire.Request) *wire.Response {
	target, err := acl.NormalizePath(req.Path)
	if err != nil {
		return reject(wire.ReasonInvalidRequest, err)
	}

	if rule, _, ok := h.Policy.SelectRule(uid, target); ok {
		if rule.Denies() || !rule.Allows(wire.OpChown) {
			return reject(wire.ReasonDenied, fmt.Errorf("uid %d may not chown %s", uid, target))
		}
	}

	if fi, err := os.Stat(target); err != nil {
		return reject(reasonFor(err), err)
	} else if fi.IsDir() {
		return reject(wire.ReasonInvalidRequest, fmt.Errorf("%s is a directory", target))
	}

	if err := os.Chown(target, req.UID, req.GID); err != nil {
		return reject(wire.ReasonIOError, err)
	}

	return &wire.Response{Version: wire.Version, OK: true}
}
