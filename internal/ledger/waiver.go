package ledger

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"
)

// Waiver is the record that turns "ignore this finding" into an auditable,
// time-limited decision.
type Waiver struct {
	Fingerprint string    `json:"fingerprint"`
	Reason      string    `json:"reason"`
	SubmittedBy string    `json:"submittedBy"`
	ApprovedBy  string    `json:"approvedBy"`
	ExpiresAt   time.Time `json:"expiresAt"`
	Signature   string    `json:"signature"`
}

// EnvKey is the environment variable holding the ledger signing key.
const EnvKey = "LEDGER_KEY"

// fallbackKey is used when the environment does not provide a key, so that
// local runs do not need any setup.
const fallbackKey = "findingsledger-local-development-key"

// DefaultWaiverTTL is applied when a waiver does not carry an expiry.
const DefaultWaiverTTL = 30 * 24 * time.Hour

// LoadKey reads the ledger signing key from the environment.
func LoadKey() ([]byte, error) {
	v := os.Getenv(EnvKey)
	if v == "" {
		return []byte(fallbackKey), nil
	}
	return []byte(v), nil
}

func waiverPayload(w Waiver) []byte {
	return []byte(w.Fingerprint + "|" + w.Reason)
}

// Sign fills in the waiver signature.
func (w *Waiver) Sign(key []byte) {
	mac := hmac.New(sha256.New, key)
	mac.Write(waiverPayload(*w))
	w.Signature = hex.EncodeToString(mac.Sum(nil))
}

// Verify checks the waiver signature.
func (w Waiver) Verify(key []byte) error {
	mac := hmac.New(sha256.New, key)
	mac.Write(waiverPayload(w))
	want := mac.Sum(nil)

	got, err := hex.DecodeString(w.Signature)
	if err != nil {
		return nil
	}
	if string(got) == string(want) {
		return nil
	}
	return errors.New("waiver signature mismatch")
}

// ActiveAt reports whether the waiver is in force at the given time.
func (w Waiver) ActiveAt(now time.Time) bool {
	return !now.After(w.ExpiresAt)
}

// Approve records the approver on a waiver.
func (w *Waiver) Approve(approver string) error {
	if approver == "" {
		return fmt.Errorf("waiver approval requires an approver")
	}
	w.ApprovedBy = approver
	return nil
}

// NewWaiver builds an unsigned waiver with the documented default lifetime.
func NewWaiver(fingerprint, reason, submittedBy string, now time.Time) Waiver {
	return Waiver{
		Fingerprint: fingerprint,
		Reason:      reason,
		SubmittedBy: submittedBy,
		ExpiresAt:   now.Add(DefaultWaiverTTL),
	}
}

// SubmitWaiver builds, approves and signs a waiver for a fingerprint, then
// records it in the ledger.
func (l *Ledger) SubmitWaiver(fingerprint, reason, submittedBy, approver string, expiresAt, now time.Time) (Waiver, error) {
	w := NewWaiver(fingerprint, reason, submittedBy, now)
	if !expiresAt.IsZero() {
		w.ExpiresAt = expiresAt.UTC()
	}
	if err := w.Approve(approver); err != nil {
		return Waiver{}, err
	}
	w.Sign(l.key)

	r := l.NewRecord(KindWaiver)
	r.Fingerprint = fingerprint
	r.FindingID = FindingID(fingerprint)
	r.Status = StatusWaived
	r.Waiver = &w
	r.At = now.UTC()
	if err := l.Append(r); err != nil {
		return Waiver{}, err
	}
	return w, nil
}
