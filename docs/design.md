# Design

This document states the contract the library is expected to uphold. It
describes required behaviour and invariants, not an implementation.

## 1. Threat model

Three parties are considered:

- **Writers** — services that produce audit records. They hold the policy and
  the keystore.
- **Readers** — anyone with access to the journal directory, but not the
  keystore.
- **Operators** — people allowed to retrieve a redacted value, in pairs.

Readers must learn nothing about redacted values beyond the fact that a value
exists and which salt opens it. Writers must be unable to publish a record
whose chain does not cover what was written. Operators must not be able to
read a value without a second person and without leaving a trace.

## 2. Invariants

### 2.1 Determinism

Encoding a record is a pure function of its logical contents. No map
iteration order, no locale, no platform and no process state may influence the
bytes. Two encodings of equal records are byte-identical.

### 2.2 Faithful text

The journal encodes strings as UTF-8 without escapes beyond what JSON
requires. Text written by the caller is text read back by the reader.

### 2.3 Total coverage

The stored hash covers every member of the record other than `hash` itself:
`seq`, `actor`, `action`, `ts`, `prev_hash`, and every leaf under `fields`,
including the commitment metadata of redacted leaves. A change to any covered
byte is detectable.

### 2.4 Explicit disclosure only

A leaf is written in plaintext only when a rule explicitly selects `keep`.
Every other leaf is redacted. This holds for empty policies and for policies
that do not mention a field.

### 2.5 Precise matching

A rule addresses the field it names and no other. Path segments are compared
whole, arrays are addressed elementwise, and case does not create a bypass.

### 2.6 Separation of secrets

The journal never contains a salt or a plaintext of a redacted leaf. The
keystore is a separate directory with its own protection boundary and can be
withheld from readers independently.

### 2.7 Two-person rule

A redaction is reversible only with an approval from an identity distinct
from the requester, bound to that exact record and field, single-use, and
covered by an audit record.

### 2.8 Trace before disclosure

No plaintext reaches a caller before the audit record describing that
disclosure has been successfully appended. A failed audit write means no
disclosure.

### 2.9 Crash safety

Index and data file agree about what has been committed. An interrupted
process may lose a tail of records, but it may not leave the index claiming
records the data file lacks, and it may not report a written record as
corrupt.

### 2.10 Verification is actionable

Verification reports the first failing sequence number and fails loudly. A
journal that has been altered must not verify successfully.

## 3. Component responsibilities

### 3.1 `internal/record`

Owns the value model, canonical encoding, the flattening of the field tree to
leaf paths, the commitment function, the sealing of originals, and the record
hash. It is the only package that decides what bytes reach the journal.

### 3.2 `internal/policy`

Owns the rule grammar and matching. It answers, for one concrete leaf path,
"redact or keep". It does not touch values.

### 3.3 `internal/keystore`

Owns salt generation and storage. An entry is created once, named by `salt_id`,
and contains the salt and the sealed original. Readers of the journal are not
expected to have access to this directory.

### 3.4 `internal/journal`

Owns the append path, the index, the durability ordering, and verification.
Appending assigns `seq` and `prev_hash`, computes the hash, and persists the
record. Verification re-derives everything except the stored hash and
compares.

### 3.5 `internal/unredact`

Owns approval, token binding, single use, and the ordered retrieval workflow.
It depends on the journal for the target record and the audit record, and on
the keystore for the salt and sealed original.

### 3.6 `cmd/maskseal`

Thin front end. Exit status reflects the library result: `verify` exits
non-zero when verification fails.

## 4. Data flow

```
caller fields ──▶ policy match ──▶ redact leaves ──▶ commitment + keystore entry
                                                   │
                                                   ▼
                                    record ──▶ hash ──▶ journal.log / journal.idx
```

```
requester ──▶ approval (second identity) ──▶ token
                                               │
                                               ▼
                            verify + consume ──▶ locate leaf ──▶ read keystore
                                               │                       │
                                               │                       ▼
                                               │                   open value
                                               ▼                       │
                                      append audit record ◀────────────┘
                                               │
                                               ▼
                                         deliver plaintext
```

## 5. Out of scope

- Network transport and remote keystores.
- Key rotation and re-encryption of old entries.
- Multi-writer concurrency across processes; a journal directory is owned by
  one process at a time. Concurrent appends from many goroutines within one
  process are in scope.
- Retention and deletion of journal entries.
