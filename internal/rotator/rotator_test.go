package rotator

import (
	"context"
	"crypto/tls"
	"sync/atomic"
	"testing"
	"time"

	"certwhirl/internal/loader"
)

func fixturePair(t *testing.T) *loader.Pair {
	t.Helper()
	pair, err := loader.LoadPair(loader.Config{
		CertFile: "../../testdata/good/leaf.pem",
		KeyFile:  "../../testdata/good/leaf-key.pem",
	})
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return pair
}

func TestNewPublishesInitialCertificate(t *testing.T) {
	t.Parallel()

	pair := fixturePair(t)
	r, err := New(Config{
		Load: func(context.Context) (*loader.Pair, error) { return pair, nil },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cfg := r.TLSConfig()
	if cfg == nil {
		t.Fatal("TLSConfig returned nil")
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("published %d certificates, want 1", len(cfg.Certificates))
	}
}

func TestCheckAndRotateLeavesFreshCertificateAlone(t *testing.T) {
	t.Parallel()

	pair := fixturePair(t)
	var loads atomic.Int32
	r, err := New(Config{
		Load: func(context.Context) (*loader.Pair, error) {
			loads.Add(1)
			return pair, nil
		},
		Clock: fixedClock{at: time.Date(2030, 6, 1, 0, 0, 0, 0, time.UTC)},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := loads.Load(); got != 1 {
		t.Fatalf("load calls after New = %d, want 1", got)
	}
	if err := r.CheckAndRotate(context.Background()); err != nil {
		t.Fatalf("CheckAndRotate: %v", err)
	}
	if got := loads.Load(); got != 1 {
		t.Fatalf("load calls after a check of a fresh certificate = %d, want 1", got)
	}
}

func TestTLSConfigSurvivesNewCertificate(t *testing.T) {
	t.Parallel()

	pair := fixturePair(t)
	var loads atomic.Int32
	r, err := New(Config{
		Load: func(context.Context) (*loader.Pair, error) {
			loads.Add(1)
			return pair, nil
		},
		Clock: fixedClock{at: time.Date(2030, 6, 1, 0, 0, 0, 0, time.UTC)},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	before := r.TLSConfig()
	if before == nil {
		t.Fatal("TLSConfig returned nil before rotation")
	}
	if err := r.CheckAndRotate(context.Background()); err != nil {
		t.Fatalf("CheckAndRotate: %v", err)
	}
	after := r.TLSConfig()
	if after == nil {
		t.Fatal("TLSConfig returned nil after rotation check")
	}
	if after.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %x, want %x", after.MinVersion, tls.VersionTLS12)
	}
}

func TestNewRejectsMissingLoader(t *testing.T) {
	t.Parallel()

	if _, err := New(Config{}); err == nil {
		t.Fatal("expected an error when Load is nil")
	}
}

func TestStopWithoutStart(t *testing.T) {
	t.Parallel()

	pair := fixturePair(t)
	r, err := New(Config{
		Load: func(context.Context) (*loader.Pair, error) { return pair, nil },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	r.Stop()
}

func TestStartStopsCleanly(t *testing.T) {
	t.Parallel()

	pair := fixturePair(t)
	r, err := New(Config{
		Load:          func(context.Context) (*loader.Pair, error) { return pair, nil },
		CheckInterval: 10 * time.Millisecond,
		Clock:         fixedClock{at: time.Date(2030, 6, 1, 0, 0, 0, 0, time.UTC)},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	r.Start(context.Background())
	r.Stop()
}

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }
