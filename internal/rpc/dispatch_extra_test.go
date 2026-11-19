package rpc

import (
	"context"
	"errors"
	"testing"

	"peerloom/internal/identity"
)

func TestOpenStreamRunsTheSameAuthorizationAsUnary(t *testing.T) {
	denied := map[string]bool{
		"stranger /ledger.v1.Ledger/Debit": true,
	}
	d := newDispatcher(t, testAuthorizer(denied))
	d.services["ledger.v1.Ledger"].Streaming["Debit"] = func(ctx context.Context, recv func() ([]byte, error), send func([]byte) error) error {
		t.Fatal("streaming handler must not run when authorization fails")
		return nil
	}
	ctx := withPeer(identity.Identity{TrustDomain: "corp", Service: "stranger"})

	recv := func() ([]byte, error) {
		t.Fatal("recv must not be called when authorization fails")
		return nil, nil
	}
	send := func([]byte) error {
		t.Fatal("send must not be called when authorization fails")
		return nil
	}

	err := d.OpenStream(ctx, "/ledger.v1.Ledger/Debit", recv, send)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("OpenStream err = %v, want ErrPermissionDenied", err)
	}

	if _, err := d.InvokeUnary(ctx, "/ledger.v1.Ledger/Debit", nil); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("InvokeUnary err = %v, want ErrPermissionDenied", err)
	}
}

func TestOpenStreamRequiresIdentity(t *testing.T) {
	d := newDispatcher(t, testAuthorizer(nil))

	recv := func() ([]byte, error) {
		t.Fatal("recv must not be called without an identity")
		return nil, nil
	}
	send := func([]byte) error { return nil }

	if err := d.OpenStream(context.Background(), "/ledger.v1.Ledger/Tail", recv, send); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("OpenStream err = %v, want ErrUnauthenticated", err)
	}
}

func TestOpenStreamAllowsPermittedPeer(t *testing.T) {
	d := newDispatcher(t, testAuthorizer(nil))
	ctx := withPeer(identity.Identity{TrustDomain: "corp", Service: "payments"})

	msgs := []string{"a", "b"}
	idx := 0
	recv := func() ([]byte, error) {
		if idx >= len(msgs) {
			return nil, errors.New("done")
		}
		m := msgs[idx]
		idx++
		return []byte(m), nil
	}
	var got []string
	send := func(b []byte) error {
		got = append(got, string(b))
		return nil
	}

	if err := d.OpenStream(ctx, "/ledger.v1.Ledger/Tail", recv, send); err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	if len(got) != 2 || got[0] != "tail:a" || got[1] != "tail:b" {
		t.Errorf("got %v, want [tail:a tail:b]", got)
	}
}
