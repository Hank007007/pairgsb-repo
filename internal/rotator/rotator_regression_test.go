package rotator

import (
	"context"
	"sync"
	"testing"
	"time"

	"certwhirl/internal/loader"
)

type regressionClock struct{ at time.Time }

func (c regressionClock) Now() time.Time { return c.at }

func loadRegressionPair(t *testing.T, dir string) *loader.Pair {
	t.Helper()
	pair, err := loader.LoadPair(loader.Config{
		CertFile: "../../testdata/" + dir + "/leaf.pem",
		KeyFile:  "../../testdata/" + dir + "/leaf-key.pem",
	})
	if err != nil {
		t.Fatalf("load %s fixture: %v", dir, err)
	}
	return pair
}

func TestCheckAndRotateIsDueWithinLeadTime(t *testing.T) {
	t.Parallel()

	pair := loadRegressionPair(t, "near-expiry")
	var mu sync.Mutex
	loads := 0
	r, err := New(Config{
		Load: func(context.Context) (*loader.Pair, error) {
			mu.Lock()
			loads++
			mu.Unlock()
			return pair, nil
		},
		RotateBefore: 72 * time.Hour,
		Clock:        regressionClock{at: time.Date(2035, 1, 2, 0, 0, 0, 0, time.UTC)},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := r.CheckAndRotate(context.Background()); err != nil {
		t.Fatalf("CheckAndRotate: %v", err)
	}
	mu.Lock()
	got := loads
	mu.Unlock()
	if got != 2 {
		t.Fatalf("load calls = %d, want 2 (initial plus replacement at 48h before expiry)", got)
	}
}

func TestCheckAndRotateHonorsExactLeadTimeBoundary(t *testing.T) {
	t.Parallel()

	pair := loadRegressionPair(t, "near-expiry")
	var loads int
	r, err := New(Config{
		Load: func(context.Context) (*loader.Pair, error) {
			loads++
			return pair, nil
		},
		RotateBefore: 72 * time.Hour,
		Clock:        regressionClock{at: time.Date(2035, 1, 1, 0, 0, 0, 0, time.UTC)},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := r.CheckAndRotate(context.Background()); err != nil {
		t.Fatalf("CheckAndRotate at exactly the lead time: %v", err)
	}
	if loads != 1 {
		t.Fatalf("load calls at exactly the lead time = %d, want 1", loads)
	}

	r.cfg.Clock = regressionClock{at: time.Date(2035, 1, 1, 0, 0, 0, 1, time.UTC)}
	if err := r.CheckAndRotate(context.Background()); err != nil {
		t.Fatalf("CheckAndRotate just past the lead time: %v", err)
	}
	if loads != 2 {
		t.Fatalf("load calls just past the lead time = %d, want 2", loads)
	}
}

func TestFailedReplacementKeepsPreviousCertificate(t *testing.T) {
	t.Parallel()

	good := loadRegressionPair(t, "good")
	bad := &loader.Pair{
		CertPEM:  []byte("not a pem certificate"),
		KeyPEM:   good.KeyPEM,
		Leaf:     good.Leaf,
		Chain:    good.Chain,
		LoadedAt: good.LoadedAt,
	}

	var loads int
	r, err := New(Config{
		Load: func(context.Context) (*loader.Pair, error) {
			loads++
			if loads == 1 {
				return good, nil
			}
			return bad, nil
		},
		Clock: regressionClock{at: time.Date(2036, 1, 1, 0, 0, 0, 0, time.UTC)},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	before := r.TLSConfig()
	if before == nil || len(before.Certificates) != 1 {
		t.Fatalf("initial TLS config has %d certificates, want 1", len(before.Certificates))
	}
	if err := r.CheckAndRotate(context.Background()); err == nil {
		t.Fatal("expected the malformed replacement to fail")
	}
	after := r.TLSConfig()
	if after == nil {
		t.Fatal("TLSConfig returned nil after a failed replacement")
	}
	if len(after.Certificates) != 1 {
		t.Fatalf("published certificates after a failed replacement = %d, want 1", len(after.Certificates))
	}
	if after != before {
		t.Fatal("failed replacement changed the published TLS config")
	}
}
