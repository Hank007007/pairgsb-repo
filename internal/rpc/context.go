// Package rpc is a tiny request dispatcher with peer authorization built in.
//
// It is intentionally not a full RPC framework: the transport framing lives in
// the commands, while this package owns identity propagation, authorization
// and the unary/streaming distinction.
package rpc

import (
	"context"
	"errors"
	"fmt"

	"peerloom/internal/identity"
)

// Errors returned to callers.
var (
	ErrUnauthenticated  = errors.New("rpc: peer is not authenticated")
	ErrPermissionDenied = errors.New("rpc: permission denied")
	ErrUnknownMethod    = errors.New("rpc: unknown method")
)

// Status is the wire-level outcome of a call.
type Status int

// Status values.
const (
	StatusOK Status = iota
	StatusUnauthenticated
	StatusPermissionDenied
	StatusUnknownMethod
	StatusInternal
)

// String names a status for logs and wire responses.
func (s Status) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusUnauthenticated:
		return "unauthenticated"
	case StatusPermissionDenied:
		return "permission_denied"
	case StatusUnknownMethod:
		return "unknown_method"
	default:
		return "internal"
	}
}

// Error wraps a status with an optional message.
type Error struct {
	Status  Status
	Message string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return e.Status.String()
	}
	return fmt.Sprintf("%s: %s", e.Status, e.Message)
}

// StatusOf maps an error to a Status.
func StatusOf(err error) Status {
	var re *Error
	if errors.As(err, &re) {
		return re.Status
	}
	switch {
	case err == nil:
		return StatusOK
	case errors.Is(err, ErrUnauthenticated):
		return StatusUnauthenticated
	case errors.Is(err, ErrPermissionDenied):
		return StatusPermissionDenied
	case errors.Is(err, ErrUnknownMethod):
		return StatusUnknownMethod
	default:
		return StatusInternal
	}
}

type identityKey struct{}

// NewContext attaches a peer identity to ctx.
func NewContext(ctx context.Context, id identity.Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}

// PeerFromContext returns the peer identity stored in ctx.
func PeerFromContext(ctx context.Context) (identity.Identity, bool) {
	id, ok := ctx.Value(identityKey{}).(identity.Identity)
	return id, ok
}
