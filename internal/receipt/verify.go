package receipt

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"time"

	"beaconproof/internal/tsa"
)

// CheckContext carries the data shared by every receipt check.
type CheckContext struct {
	// PublicKey verifies the authority signature.
	PublicKey ed25519.PublicKey
}

// Finding is one receipt verification problem.
//
// Index is the 1-based position of the receipt in the order receipts are
// checked; Field names the offending field.
type Finding struct {
	Index  int
	From   int64
	To     int64
	Field  string
	Detail string
}

func (f Finding) String() string {
	return fmt.Sprintf("receipt #%d [%d, %d]: %s: %s", f.Index, f.From, f.To, f.Field, f.Detail)
}

// VerifyReceipt checks one receipt against the local chain.
//
// The authority signature is validated against ctx.PublicKey.
func VerifyReceipt(index int, r Receipt, ctx CheckContext) []Finding {
	problem := func(field, detail string) Finding {
		return Finding{Index: index, From: r.FromSeq, To: r.ToSeq, Field: field, Detail: detail}
	}

	if r.FromSeq <= 0 || r.ToSeq < r.FromSeq {
		return []Finding{problem("range", fmt.Sprintf("[%d, %d] is not a valid interval", r.FromSeq, r.ToSeq))}
	}
	if r.Root == "" {
		return []Finding{problem("root", "receipt has no root")}
	}
	if r.GenTime == "" {
		return []Finding{problem("genTime", "receipt has no generation time")}
	}
	if err := verifySignature(r, ctx.PublicKey); err != nil {
		return []Finding{problem("sig", err.Error())}
	}
	return nil
}

// VerifyAll checks every receipt, in the order given, and returns the combined
// findings.
func VerifyAll(receipts []Receipt, ctx CheckContext) []Finding {
	var out []Finding
	for i, r := range receipts {
		out = append(out, VerifyReceipt(i+1, r, ctx)...)
	}
	return out
}

func verifySignature(r Receipt, pub ed25519.PublicKey) error {
	if len(pub) == 0 {
		return fmt.Errorf("no authority public key configured")
	}
	raw, err := base64.StdEncoding.DecodeString(r.Sig)
	if err != nil {
		return fmt.Errorf("signature is not valid base64")
	}
	if !ed25519.Verify(pub, tsa.SignedMessage(r.Root, r.GenTime), raw) {
		return fmt.Errorf("signature does not match receipt contents")
	}
	return nil
}

// Age reports how long before ref the receipt was issued.
func (r Receipt) Age(ref time.Time) (time.Duration, error) {
	gen, err := time.Parse(time.RFC3339, r.GenTime)
	if err != nil {
		return 0, fmt.Errorf("parse genTime: %w", err)
	}
	return ref.Sub(gen), nil
}
