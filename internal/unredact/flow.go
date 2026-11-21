package unredact

import (
	"errors"
	"fmt"
	"time"

	"maskseal-journal/internal/keystore"
	"maskseal-journal/internal/record"
)

// Log is the part of the journal the unredaction workflow needs.
type Log interface {
	Get(seq int64) (record.Record, error)
	Append(record.Record) (record.Record, error)
}

// Result is the outcome of a successful unredaction.
type Result struct {
	Seq       int64
	Field     string
	Plaintext string
	Salt      []byte
	AuditSeq  int64
}

// ErrFieldNotRedacted is returned when the requested field holds no
// commitment.
var ErrFieldNotRedacted = errors.New("unredact: field is not redacted")

// Unredact authorises req, opens the committed value, and records the
// unredaction in the journal. deliver is called with the recovered plaintext.
func Unredact(log Log, salts *keystore.Store, approver *Approver, registry *Registry, req Request, tok *Token, deliver func(string)) (Result, error) {
	var result Result
	result.Seq = req.Seq
	result.Field = req.Field

	if err := approver.VerifyToken(tok, req); err != nil {
		return Result{}, err
	}
	if err := registry.Consume(tok); err != nil {
		return Result{}, err
	}

	rec, err := log.Get(req.Seq)
	if err != nil {
		return Result{}, err
	}
	value, ok := record.Lookup(rec.Fields, req.Field)
	if !ok {
		return Result{}, fmt.Errorf("unredact: field %q not present in record %d", req.Field, req.Seq)
	}
	redacted, ok := value.(*record.Redacted)
	if !ok || redacted == nil {
		return Result{}, ErrFieldNotRedacted
	}

	entry, err := salts.Get(keystore.SaltID(redacted.SaltID))
	if err != nil {
		return Result{}, err
	}
	result.Salt = entry.Salt

	plaintext, err := record.Open(entry.Salt, redacted.SaltID, req.Field, entry.Ciphertext)
	if err != nil {
		return result, err
	}
	result.Plaintext = record.RenderPlain(plaintext)

	if deliver != nil {
		deliver(result.Plaintext)
	}

	audit := record.Record{
		Actor:  req.Requester.ID,
		Action: "unredact",
		Time:   time.Now().UTC(),
		Fields: map[string]any{
			"target_seq":   req.Seq,
			"target_field": req.Field,
			"requester":    req.Requester.ID,
			"approver":     tok.ApproverID,
			"token_id":     tok.ID,
		},
	}
	appended, err := log.Append(audit)
	if err != nil {
		return result, nil
	}
	result.AuditSeq = appended.Seq
	return result, nil
}
