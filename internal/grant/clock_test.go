package grant

import (
	"testing"
	"time"
)

func TestWithinSkewZeroExpiryNeverExpires(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	for _, kind := range []Kind{KindOIDCAccessToken, KindServiceAccount, KindDelegated} {
		if !WithinSkew(kind, now, time.Time{}) {
			t.Errorf("%s: zero expiry reported as expired", kind)
		}
	}
}

func TestWithinSkewFreshCredential(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	exp := now.Add(10 * time.Minute)
	for _, kind := range []Kind{KindOIDCAccessToken, KindServiceAccount, KindDelegated} {
		if !WithinSkew(kind, now, exp) {
			t.Errorf("%s: fresh credential reported as expired", kind)
		}
	}
}

func TestWithinSkewWellPastExpiry(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	exp := now.Add(-time.Hour)
	for _, kind := range []Kind{KindOIDCAccessToken, KindServiceAccount, KindDelegated} {
		if WithinSkew(kind, now, exp) {
			t.Errorf("%s: long-expired credential reported as usable", kind)
		}
	}
}
