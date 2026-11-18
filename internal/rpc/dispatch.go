package rpc

import (
	"context"
	"fmt"
	"strings"
)

// Dispatcher routes a fully qualified method to a registered handler.
type Dispatcher struct {
	az       Authorizer
	services map[string]*Service
}

// NewDispatcher returns a dispatcher that consults az for every call.
func NewDispatcher(az Authorizer) *Dispatcher {
	return &Dispatcher{az: az, services: make(map[string]*Service)}
}

// Register adds a service to the dispatcher.
func (d *Dispatcher) Register(s *Service) error {
	if s == nil || s.Name == "" {
		return fmt.Errorf("rpc: service has no name")
	}
	if _, dup := d.services[s.Name]; dup {
		return fmt.Errorf("rpc: service %q registered twice", s.Name)
	}
	d.services[s.Name] = s
	return nil
}

// splitMethod splits "/service/method" into its two parts.
func splitMethod(full string) (service, method string, ok bool) {
	if !strings.HasPrefix(full, "/") {
		return "", "", false
	}
	rest := strings.TrimPrefix(full, "/")
	idx := strings.LastIndexByte(rest, '/')
	if idx <= 0 || idx == len(rest)-1 {
		return "", "", false
	}
	return rest[:idx], rest[idx+1:], true
}

func (d *Dispatcher) resolve(full string) (*Service, string, error) {
	svc, method, ok := splitMethod(full)
	if !ok {
		return nil, "", ErrUnknownMethod
	}
	s, found := d.services[svc]
	if !found {
		return nil, "", ErrUnknownMethod
	}
	return s, method, nil
}

// InvokeUnary runs a unary call.
func (d *Dispatcher) InvokeUnary(ctx context.Context, fullMethod string, req []byte) ([]byte, error) {
	s, method, err := d.resolve(fullMethod)
	if err != nil {
		return nil, err
	}
	h, ok := s.Unary[method]
	if !ok {
		return nil, ErrUnknownMethod
	}
	return unaryCall(ctx, d.az, fullMethod, h, req)
}

// OpenStream runs a streaming call.
func (d *Dispatcher) OpenStream(ctx context.Context, fullMethod string, recv func() ([]byte, error), send func([]byte) error) error {
	s, method, err := d.resolve(fullMethod)
	if err != nil {
		return err
	}
	h, ok := s.Streaming[method]
	if !ok {
		return ErrUnknownMethod
	}
	return streamCall(ctx, d.az, fullMethod, h, recv, send)
}
