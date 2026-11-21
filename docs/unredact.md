# Unredaction

`maskseal unredact` is the operational path that recovers one redacted field
from one record. It is deliberately narrow: two people, one field, one use of
one approval, and an audit entry for every retrieval.

## 1. Identities

Each participant is an `Identity`:

| Member | Meaning |
| --- | --- |
| `ID` | stable account identifier (the only identity key) |
| `DisplayName` | presentation label |

Identity comparisons that decide whether two people are the same person use
`ID`, never `DisplayName`. `DisplayName` is free-form and can be duplicated,
re-cased, or changed by the user.

## 2. Approval

A request names a record `seq`, a field path, and the requester. A second
person approves it and a token is issued.

### 2.1 Who may approve

The approver MUST be a different person from the requester. "Different" is
decided on the account identity:

- two identities with the same `ID` are the same person;
- differences in case, surrounding whitespace, or display name do not make
  two identities different;
- an approver whose `ID` is empty is rejected.

If the approver and requester are the same person, approval fails with
`ErrSelfApproval` and no token is issued.

### 2.2 What the token is bound to

A token authorises **exactly one** request:

- the record `seq`;
- the field path;
- the requester identity;
- the approver identity.

A token approved for `seq 7, field user.password` MUST NOT authorise
`seq 7, field items[0].ssn`, and MUST NOT authorise `seq 8` even for the same
field. Presenting a token against a request with a different seq or field is
rejected with `ErrBadToken`.

### 2.3 Signature

Tokens carry a signature over their binding fields and issue time. Signature
verification:

- returns success only when the presented signature is byte-for-byte equal to
  the expected signature;
- compares in a way that does not branch on the position of the first
  differing byte;
- returns failure for inputs of different lengths.

A signature check must never succeed merely because lengths differ or because
the comparison was short-circuited.

### 2.4 Single use

A token can be used once. After a successful consumption the token MUST be
rejected with `ErrTokenReused`. Tokens are identified by their token id; the
same token presented twice is rejected even if everything else about the
request is unchanged.

## 3. Retrieval order

The workflow has a required order. The plaintext MUST NOT be released to the
caller until every step has succeeded, including the audit write.

1. verify the token against the request (§2.2, §2.3);
2. consume the token (§2.4);
3. read the record and locate the commitment leaf;
4. read the keystore entry named by the leaf's `salt_id`; the entry contains
   both the salt and the sealed original;
5. open the sealed original with the salt;
6. append the audit entry for this unredaction;
7. only after the audit append succeeds, deliver the plaintext to the caller.

The audit entry records at least: the target `seq`, the target field path, the
requester identity, the approver identity, and the token id. It is appended
through the journal, so it is itself hashed and chained like any other record.

## 4. Failure and rollback

The recovery is all-or-nothing from the caller's point of view.

- If any step before the audit append fails, no plaintext is delivered and
  the caller receives a non-nil error. The salt and plaintext must not be
  present in the returned value.
- If the audit append fails, the call returns a non-nil error, no plaintext is
  delivered, and no salt is returned to the caller. A retrieval that leaves no
  trace must not happen.
- A token consumed by a request that then fails is not silently re-usable:
  the next attempt needs a fresh approval.

The salt is read into memory only as part of opening the value. It is not
returned to the caller as a separate output before the audit entry exists.

## 5. Observability

Every successful retrieval produces exactly one audit record whose action is
`unredact`. The presence of that record, and the chain that contains it, is
what makes the retrieval evidenceable after the fact.
