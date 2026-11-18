package rpc

import (
	"context"
)

// Authorizer decides whether a peer service may invoke a method.
type Authorizer interface {
	// Check returns nil when the call is allowed and an error otherwise.
	Check(peer, method string) error
}

// AuthorizerFunc adapts a function to Authorizer.
type AuthorizerFunc func(peer, method string) error

// Check implements Authorizer.
func (f AuthorizerFunc) Check(peer, method string) error {
	return f(peer, method)
}

// HandlerFunc serves a unary call.
type HandlerFunc func(ctx context.Context, req []byte) ([]byte, error)

// StreamHandler serves a streaming call. recv is called repeatedly until it
// returns io.EOF; send delivers one response message.
type StreamHandler func(ctx context.Context, recv func() ([]byte, error), send func([]byte) error) error

// Service is a named collection of methods.
type Service struct {
	// Name is the fully qualified service name, e.g. "ledger.v1.Ledger".
	Name string
	// Unary maps method names, e.g. "Debit", to handlers.
	Unary map[string]HandlerFunc
	// Streaming maps method names to streaming handlers.
	Streaming map[string]StreamHandler
}

// MethodName renders the authorization name of a method.
func MethodName(service, method string) string {
	return "/" + service + "/" + method
}

// unaryCall is the full unary pipeline: identity, authorization, handler.
func unaryCall(ctx context.Context, az Authorizer, method string, h HandlerFunc, req []byte) ([]byte, error) {
	id, ok := PeerFromContext(ctx)
	if !ok || id.IsZero() {
		return nil, ErrUnauthenticated
	}
	if err := az.Check(id.Service, method); err != nil {
		return nil, ErrPermissionDenied
	}
	return h(ctx, req)
}

// streamCall is the streaming pipeline. The stream is established before the
// handler is invoked.
func streamCall(ctx context.Context, az Authorizer, method string, h StreamHandler, recv func() ([]byte, error), send func([]byte) error) error {
	return h(ctx, recv, send)
}
