# Waivers

This document is normative. A waiver is the only way to stop a finding from
being reported while it is still present. It exists so that "ignore this" is a
reviewable decision rather than a silent suppression.

## §1. Contents

A waiver record carries:

| Field | Meaning |
|---|---|
| `fingerprint` | the finding being waived |
| `reason` | free text explaining why the risk is accepted |
| `submittedBy` | the person asking for the waiver |
| `approvedBy` | the person who approved it |
| `expiresAt` | when the waiver stops applying, RFC 3339 UTC |
| `signature` | MAC over the waiver, see §4 |

All six fields are required. A waiver with an empty reason, submitter or
approver is rejected.

## §2. Two-person rule

Approval must come from someone other than the submitter. A waiver whose
`approvedBy` equals its `submittedBy` is not approved and must be rejected when
it is created and when it is verified. Self-approval is never valid, regardless
of the submitter's role.

## §3. Expiry

A waiver applies from the moment it is recorded until `expiresAt`, inclusive of
the instant named. After that instant it has no effect, even though the record
remains in the ledger. An expired waiver does not need to be removed, revoked or
re-approved to stop applying; it simply stops.

Extensions are new waiver records. The old record and its expiry stay in the
history.

## §4. Signature

Every waiver is signed with the ledger key using HMAC-SHA256. The signature is
lowercase hex over the UTF-8 encoding of:

```
fingerprint + "|" + reason + "|" + submittedBy + "|" + approvedBy + "|" + expiresAt.RFC3339
```

All fields that affect whether the waiver applies are covered. In particular,
changing the expiry or the approver of a stored waiver invalidates its
signature; an operator cannot extend a waiver or swap its approver without
writing and signing a new record.

Verification compares the recomputed MAC with the stored signature in constant
time, and treats any signature that cannot be decoded as a failure. A waiver
whose signature does not verify must be treated as though it did not exist: it
does not waive anything.

## §5. The ledger key

The key comes from the `LEDGER_KEY` environment variable. There is no default
key. If the variable is missing or empty, `scanctl` refuses to start and exits
non-zero rather than falling back to a key that is the same for every
installation.

The key is shared only between the ledger writers and the verification job; it
is not stored in the repository.

## §6. What a waiver does and does not do

A valid, unexpired waiver:

- makes the finding's effective state `waived`;
- keeps the finding out of the `diff` new-findings list;
- does not remove the finding's history or its records.

Once the waiver expires, the finding is treated as `open` again and appears in
the next `diff` output at its underlying severity, unless a new waiver has been
recorded.
