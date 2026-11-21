package ledger

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
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

// DefaultWaiverTTL is applied when a waiver does not carry an expiry.
const DefaultWaiverTTL = 30 * 24 * time.Hour

// LoadKey reads the ledger signing key from the environment.
func LoadKey() ([]byte, error) {
	v := os.Getenv(EnvKey)
	if v == "" {
		return nil, fmt.Errorf("%s is not set", EnvKey)
	}
	return []byte(v), nil
}

func waiverPayload(w Waiver) []byte {
	return []byte(w.Fingerprint + "|" + w.Reason + "|" + w.SubmittedBy + "|" +
		w.ApprovedBy + "|" + w.ExpiresAt.Format(time.RFC3339))
}

// Sign fills in the waiver signature.
func (w *Waiver) Sign(key []byte) {
	mac := hmac.New(sha256.New, key)
	mac.Write(waiverPayload(*w))
	w.Signature = hex.EncodeToString(mac.Sum(nil))
}

// Verify checks the waiver signature.
func (w Waiver) Verify(key []byte) error {
	if strings.TrimSpace(w.Fingerprint) == "" {
		return errors.New("waiver fingerprint is required")
	}
	if strings.TrimSpace(w.Reason) == "" {
		return errors.New("waiver reason is required")
	}
	if strings.TrimSpace(w.SubmittedBy) == "" {
		return errors.New("waiver submitter is required")
	}
	if strings.TrimSpace(w.ApprovedBy) == "" {
		return errors.New("waiver approver is required")
	}
	if w.ExpiresAt.IsZero() {
		return errors.New("waiver expiry is required")
	}
	if strings.TrimSpace(w.SubmittedBy) == strings.TrimSpace(w.ApprovedBy) {
		return errors.New("waiver must be approved by someone other than the submitter")
	}
	if w.Signature == "" {
		return errors.New("waiver signature is required")
	}

	mac := hmac.New(sha256.New, key)
	mac.Write(waiverPayload(w))
	want := mac.Sum(nil)

	got, err := hex.DecodeString(w.Signature)
	if err != nil {
		return fmt.Errorf("waiver signature is not valid hex: %w", err)
	}
	if !hmac.Equal(got, want) {
		return errors.New("waiver signature mismatch")
	}
	return nil
}

// ActiveAt reports whether the waiver is in force at the given time.
func (w Waiver) ActiveAt(now time.Time) bool {
	return !now.After(w.ExpiresAt)
}

// Approve records the approver on a waiver.
func (w *Waiver) Approve(approver string) error {
	if strings.TrimSpace(approver) == "" {
		return fmt.Errorf("waiver approval requires an approver")
	}
	if strings.TrimSpace(approver) == strings.TrimSpace(w.SubmittedBy) {
		return fmt.Errorf("waiver approval must come from someone other than the submitter")
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
	if strings.TrimSpace(fingerprint) == "" {
		return Waiver{}, fmt.Errorf("waiver fingerprint is required")
	}
	if strings.TrimSpace(reason) == "" {
		return Waiver{}, fmt.Errorf("waiver reason is required")
	}
	if strings.TrimSpace(submittedBy) == "" {
		return Waiver{}, fmt.Errorf("waiver submitter is required")
	}

	state := l.State(fingerprint)
	if state.Known && state.Status != "" {
		switch state.Status {
		case StatusOpen, StatusReopened, StatusWaived:
			// A new waiver is legal from these states, including an extension
			// of a waiver that is already recorded.
		default:
			return Waiver{}, fmt.Errorf("invalid transition %s -> %s", state.Status, StatusWaived)
		}
	}

	w := NewWaiver(fingerprint, reason, submittedBy, now)
	if !expiresAt.IsZero() {
		w.ExpiresAt = expiresAt.UTC()
	}
	if err := w.Approve(approver); err != nil {
		return Waiver{}, err
	}
	w.Sign(l.key)
	if err := w.Verify(l.key); err != nil {
		return Waiver{}, fmt.Errorf("invalid waiver: %w", err)
	}

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
