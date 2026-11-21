package receipt

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"beaconproof/internal/chain"
	"beaconproof/internal/merkle"
	"beaconproof/internal/tsa"
)

// CheckContext carries the data shared by every receipt check.
type CheckContext struct {
	// PublicKey verifies the authority signature.
	PublicKey ed25519.PublicKey
	// Records is the local chain the receipts are checked against.
	Records []chain.Record
	// Store is an alternative source of local records.
	Store *chain.Store
	// HeadSeq is the chain head. It is derived from Records when zero.
	HeadSeq int64
	// Tolerance is how far outside an interval's time window a generation
	// time may fall. Zero selects the default of ten minutes.
	Tolerance time.Duration
}

// When no chain source (Records, Store, or HeadSeq) is supplied, only the
// receipt's internal structure and signature are checked.

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
	if f.Index <= 0 {
		return fmt.Sprintf("chain [%d, %d]: %s: %s", f.From, f.To, f.Field, f.Detail)
	}
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

	var findings []Finding
	records := recordsOf(ctx)
	bySeq, covered := intervalRecords(records, r.FromSeq, r.ToSeq)
	hasChain := ctx.Store != nil || len(ctx.Records) > 0 || ctx.HeadSeq > 0
	if hasChain && !covered {
		findings = append(findings, problem("range",
			fmt.Sprintf("[%d, %d] is not fully covered by local records", r.FromSeq, r.ToSeq)))
	}

	var rootBytes []byte
	if r.Root == "" {
		findings = append(findings, problem("root", "receipt has no root"))
	} else if raw, err := hex.DecodeString(r.Root); err != nil {
		findings = append(findings, problem("root", "root is not valid hexadecimal"))
	} else {
		rootBytes = raw
	}

	if err := verifySignature(r, ctx.PublicKey); err != nil {
		findings = append(findings, problem("sig", err.Error()))
	}

	if rootBytes != nil && hasChain && covered {
		if computed, err := recomputeRoot(bySeq, r.FromSeq, r.ToSeq); err != nil {
			findings = append(findings, problem("root", err.Error()))
		} else if !bytes.Equal(rootBytes, computed) {
			findings = append(findings, problem("root",
				fmt.Sprintf("signed root does not match records in [%d, %d]", r.FromSeq, r.ToSeq)))
		}
	}

	if r.GenTime == "" {
		findings = append(findings, problem("genTime", "receipt has no generation time"))
	} else if gen, err := time.Parse(time.RFC3339, r.GenTime); err != nil {
		findings = append(findings, problem("genTime", "generation time is not RFC3339"))
	} else if hasChain && covered {
		if detail := generationTimeProblem(gen, bySeq, r.FromSeq, r.ToSeq, ctx.Tolerance); detail != "" {
			findings = append(findings, problem("genTime", detail))
		}
	}

	return findings
}

// VerifyAll checks every receipt, in the order given, and returns the combined
// findings.
func VerifyAll(receipts []Receipt, ctx CheckContext) []Finding {
	ordered := make([]Receipt, len(receipts))
	copy(ordered, receipts)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].FromSeq != ordered[j].FromSeq {
			return ordered[i].FromSeq < ordered[j].FromSeq
		}
		return ordered[i].ToSeq < ordered[j].ToSeq
	})

	var out []Finding
	for i, r := range ordered {
		out = append(out, VerifyReceipt(i+1, r, ctx)...)
	}

	if len(ordered) > 0 && ordered[0].FromSeq > 1 {
		out = append(out, Finding{
			Index: 1,
			From:  1,
			To:    ordered[0].FromSeq - 1,
			Field: "interval",
			Detail: fmt.Sprintf("records [1, %d] are not covered by any receipt",
				ordered[0].FromSeq-1),
		})
	}

	for i := 1; i < len(ordered); i++ {
		a, b := ordered[i-1], ordered[i]
		if b.FromSeq <= a.ToSeq {
			out = append(out, Finding{
				Index: i + 1,
				From:  b.FromSeq,
				To:    a.ToSeq,
				Field: "interval",
				Detail: fmt.Sprintf("receipt #%d [%d, %d] overlaps receipt #%d [%d, %d]",
					i+1, b.FromSeq, b.ToSeq, i, a.FromSeq, a.ToSeq),
			})
			continue
		}
		if b.FromSeq > a.ToSeq+1 {
			out = append(out, Finding{
				Index: i + 1,
				From:  a.ToSeq + 1,
				To:    b.FromSeq - 1,
				Field: "interval",
				Detail: fmt.Sprintf("receipt #%d [%d, %d] and receipt #%d [%d, %d] leave a gap at [%d, %d]",
					i, a.FromSeq, a.ToSeq, i+1, b.FromSeq, b.ToSeq, a.ToSeq+1, b.FromSeq-1),
			})
		}
	}

	records := recordsOf(ctx)
	head := headSeq(ctx, records)
	if head > 0 {
		newest := int64(0)
		for _, r := range ordered {
			if r.ToSeq > newest {
				newest = r.ToSeq
			}
		}
		if head > newest {
			out = append(out, Finding{
				Index: 0,
				From:  newest + 1,
				To:    head,
				Field: "interval",
				Detail: fmt.Sprintf("%d record(s) unanchored after seq %d",
					head-newest, newest),
			})
		}
	}
	return out
}

func recordsOf(ctx CheckContext) []chain.Record {
	if len(ctx.Records) > 0 {
		return ctx.Records
	}
	if ctx.Store != nil {
		return ctx.Store.Records()
	}
	return nil
}

func headSeq(ctx CheckContext, records []chain.Record) int64 {
	if ctx.HeadSeq > 0 {
		return ctx.HeadSeq
	}
	var head int64
	for _, rec := range records {
		if rec.Seq > head {
			head = rec.Seq
		}
	}
	return head
}

func intervalRecords(records []chain.Record, from, to int64) (map[int64]chain.Record, bool) {
	if len(records) == 0 {
		return nil, false
	}
	bySeq := make(map[int64]chain.Record, len(records))
	for _, rec := range records {
		bySeq[rec.Seq] = rec
	}
	count := to - from + 1
	if count <= 0 || count > int64(len(bySeq)) {
		return bySeq, false
	}
	for seq := from; seq <= to; seq++ {
		if _, ok := bySeq[seq]; !ok {
			return bySeq, false
		}
	}
	return bySeq, true
}

func recomputeRoot(records map[int64]chain.Record, from, to int64) ([]byte, error) {
	leaves := make(map[int64][]byte, to-from+1)
	for seq := from; seq <= to; seq++ {
		rec := records[seq]
		raw, err := hex.DecodeString(rec.Hash)
		if err != nil {
			return nil, fmt.Errorf("record %d has an invalid hash", seq)
		}
		leaves[seq] = raw
	}
	root := merkle.RangeRoot(leaves, from, to)
	if root == nil {
		return nil, fmt.Errorf("cannot compute a Merkle root for [%d, %d]", from, to)
	}
	return root, nil
}

func generationTimeProblem(gen time.Time, records map[int64]chain.Record, from, to int64, tolerance time.Duration) string {
	fromTS, err := time.Parse(time.RFC3339, records[from].Time)
	if err != nil {
		return ""
	}
	toTS, err := time.Parse(time.RFC3339, records[to].Time)
	if err != nil {
		return ""
	}
	start, end := fromTS, toTS
	if end.Before(start) {
		start, end = end, start
	}
	if tolerance == 0 {
		tolerance = 10 * time.Minute
	} else if tolerance < 0 {
		tolerance = 0
	}
	if gen.Before(start.Add(-tolerance)) || gen.After(end.Add(tolerance)) {
		return fmt.Sprintf("generation time is outside the interval time window [%s, %s] +/- %s",
			start.UTC().Format(time.RFC3339Nano), end.UTC().Format(time.RFC3339Nano), tolerance)
	}
	return ""
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
