package rpc

import (
	"context"
	"errors"
	"testing"

	"peerloom/internal/identity"
)

func testAuthorizer(denied map[string]bool) Authorizer {
	return AuthorizerFunc(func(peer, method string) error {
		if denied[peer+" "+method] {
			return errors.New("denied")
		}
		return nil
	})
}

func newDispatcher(t *testing.T, az Authorizer) *Dispatcher {
	t.Helper()
	d := NewDispatcher(az)
	svc := &Service{
		Name: "ledger.v1.Ledger",
		Unary: map[string]HandlerFunc{
			"Debit": func(ctx context.Context, req []byte) ([]byte, error) {
				return append([]byte("debit:"), req...), nil
			},
		},
		Streaming: map[string]StreamHandler{
			"Tail": func(ctx context.Context, recv func() ([]byte, error), send func([]byte) error) error {
				for {
					msg, err := recv()
					if err != nil {
						return nil
					}
					if err := send(append([]byte("tail:"), msg...)); err != nil {
						return err
					}
				}
			},
		},
	}
	if err := d.Register(svc); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return d
}

func withPeer(id identity.Identity) context.Context {
	return NewContext(context.Background(), id)
}

func TestInvokeUnaryAllowsPermittedPeer(t *testing.T) {
	d := newDispatcher(t, testAuthorizer(nil))
	ctx := withPeer(identity.Identity{TrustDomain: "corp", Service: "payments"})

	out, err := d.InvokeUnary(ctx, "/ledger.v1.Ledger/Debit", []byte("42"))
	if err != nil {
		t.Fatalf("InvokeUnary: %v", err)
	}
	if string(out) != "debit:42" {
		t.Errorf("out = %q, want %q", out, "debit:42")
	}
}

func TestInvokeUnaryRejectsDeniedPeer(t *testing.T) {
	d := newDispatcher(t, testAuthorizer(map[string]bool{
		"stranger /ledger.v1.Ledger/Debit": true,
	}))
	ctx := withPeer(identity.Identity{TrustDomain: "corp", Service: "stranger"})

	if _, err := d.InvokeUnary(ctx, "/ledger.v1.Ledger/Debit", nil); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("err = %v, want ErrPermissionDenied", err)
	}
}

func TestInvokeUnaryRequiresIdentity(t *testing.T) {
	d := newDispatcher(t, testAuthorizer(nil))

	if _, err := d.InvokeUnary(context.Background(), "/ledger.v1.Ledger/Debit", nil); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
}

func TestInvokeUnknownMethod(t *testing.T) {
	d := newDispatcher(t, testAuthorizer(nil))
	ctx := withPeer(identity.Identity{TrustDomain: "corp", Service: "payments"})

	if _, err := d.InvokeUnary(ctx, "/ledger.v1.Ledger/Nope", nil); !errors.Is(err, ErrUnknownMethod) {
		t.Fatalf("err = %v, want ErrUnknownMethod", err)
	}
	if _, err := d.InvokeUnary(ctx, "not-a-method", nil); !errors.Is(err, ErrUnknownMethod) {
		t.Fatalf("err = %v, want ErrUnknownMethod", err)
	}
}

func TestOpenStreamDeliversMessages(t *testing.T) {
	d := newDispatcher(t, testAuthorizer(nil))
	ctx := withPeer(identity.Identity{TrustDomain: "corp", Service: "payments"})

	var sent []string
	var in []byte
	recv := func() ([]byte, error) {
		if in != nil {
			return nil, errors.New("done")
		}
		in = []byte("x")
		return []byte("a"), nil
	}
	send := func(b []byte) error {
		sent = append(sent, string(b))
		return nil
	}

	if err := d.OpenStream(ctx, "/ledger.v1.Ledger/Tail", recv, send); err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	if len(sent) != 1 || sent[0] != "tail:a" {
		t.Errorf("sent = %v, want [tail:a]", sent)
	}
}
