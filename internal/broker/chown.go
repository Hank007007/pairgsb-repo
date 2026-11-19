package broker

import (
	"fmt"
	"os"

	"filesentry/internal/wire"
)

// Chown applies a chown request made by uid. It is deliberately stricter than
// the other operations: the target must lie inside the caller's chown tenant
// root, must pass containment after symlink resolution, and the requested
// ids may only name the caller itself or a negative "leave unchanged" value.
func (h *Handler) Chown(uid int, target string, req *wire.Request) *wire.Response {
	rule, _, ok := h.Policy.SelectRule(uid, target)
	if !ok || rule.Denies() || !rule.Allows(wire.OpChown) {
		return reject(wire.ReasonDenied, fmt.Errorf("uid %d may not chown %s", uid, target))
	}

	tenantRoot, hasTenant := h.Policy.TenantRoot(uid, wire.OpChown)
	if !hasTenant || !containsPath(tenantRoot, target) {
		return reject(wire.ReasonDenied, fmt.Errorf("%s is outside the chown tenant root for uid %d", target, uid))
	}

	newUID, okUID := ownerForCaller(req.UID, uid)
	newGID, okGID := ownerForCaller(req.GID, uid)
	if !okUID || !okGID {
		return reject(wire.ReasonDenied, fmt.Errorf("uid %d may only assign its own ids", uid))
	}

	opened, err := resolveFile(target, tenantRoot, os.O_RDONLY, 0)
	if err != nil {
		return reject(reasonFor(err), err)
	}
	defer opened.file.Close()

	fi, err := opened.file.Stat()
	if err != nil {
		return reject(wire.ReasonIOError, err)
	}
	if fi.IsDir() {
		return reject(wire.ReasonInvalidRequest, fmt.Errorf("%s is a directory", target))
	}

	if err := fchown(opened.file, newUID, newGID); err != nil {
		return reject(wire.ReasonIOError, err)
	}

	return &wire.Response{Version: wire.Version, OK: true}
}

// ownerForCaller maps a requested owner id to the id actually applied. A
// negative value leaves the current owner unchanged; the caller's own uid is
// accepted; anything else is refused so ownership can never be handed to a
// third party.
func ownerForCaller(requested, caller int) (int, bool) {
	if requested < 0 {
		return -1, true
	}
	if requested == caller {
		return requested, true
	}
	return 0, false
}
