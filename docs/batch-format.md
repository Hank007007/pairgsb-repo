# Batch format, ordering, and digest

## §1 Envelope

A batch sent to the collector is a JSON object:

```json
{
  "batchId": "string, unique per send attempt",
  "source": "string, the relay/source identifier",
  "prevDigest": "hex string, or null for the first batch",
  "records": [ /* ordered records, see §2 */ ],
  "digest": "hex string, see §4"
}
```

`batchId` is generated fresh for each batch. A batch that is retried keeps the
same `batchId`; a new batch gets a new one.

## §2 Records

Each element of `records` carries the event data plus shipping metadata:

| Field | Meaning |
|---|---|
| `seq` | Producer sequence number, from the source line. |
| `source_ts` | Source timestamp of the event, taken from the `ts` field of the source line (normalised, see `time.md`). |
| `ship_ts` | Wall-clock time at which the relay prepared the batch. |
| `skew_ms` | `ship_ts - source_ts` in milliseconds. |
| `prev_hash` | Producer's hash link for the event, unchanged. |
| `payload` | Event payload, after redaction. |
| `meta` | Event metadata, after redaction. |

A **gap record** has `seq` set to the first dropped seq and the additional
fields `gap_seq_end`, `gap_count`, and `gap_reason`; it has no `payload`. Its
`source_ts` is the source timestamp of the last dropped event and its other
shipping fields are populated as above.

## §3 Ordering

1. Records in a batch are ordered by `(source_ts, seq)` ascending: source time
   first, producer sequence as the tie-breaker.
2. Ordering is total and stable. Two records that share a `source_ts` value must
   still be ordered by `seq`, and the order must not depend on file traversal,
   insertion, or any other incidental ordering.
3. Re-reading the same events — for example after a restart or a retransmission
   — must yield the same order and therefore the same digest.

## §4 Digest

1. The batch `digest` is a SHA-256 over a canonical serialisation of the batch
   **content**. Canonical means: a fixed field order, no insignificant
   whitespace, and unambiguous delimiters between records and fields.
2. The digest covers, for every record in order: `seq`, `source_ts`, `prev_hash`,
   `payload`, `meta`, and the gap fields when present. It covers the record
   count and, for gaps, the gap range and count. It is computed over each
   record's own canonical form, then over the sequence of those forms.
3. The digest is sensitive to: removing any record, adding any record,
   reordering any two records (including those with equal `source_ts`), changing
   any field of any record, and changing the number of records. In particular
   two records with different `seq` or different `prev_hash` but the same
   `payload` must not produce the same digest.
4. The digest does not cover `batchId`, `ship_ts`, or `skew_ms`, all of which may
   legitimately differ between a first send and a retransmission of the same
   content. A retransmission of the same records in the same order must produce
   the same digest.
5. The digest does not cover the records of other batches. Chaining is expressed
   through `prevDigest` (§5).

## §5 Digest chain

1. `prevDigest` of a batch is the `digest` of the **last batch the collector
   acknowledged**, or `null` if no batch has been acknowledged yet.
2. It is therefore a property of acknowledged delivery, not of local send
   attempts. If a batch failed and is retried, the retry carries the same
   `prevDigest` as the original attempt; if an earlier batch was never
   acknowledged, its content must not be adopted as `prevDigest`.
3. The collector can verify the chain by comparing each batch's `prevDigest` to
   the digest of the batch it acknowledged immediately before.

## §6 Idempotency

The collector identifies a batch by `batchId`. Retransmitting a batch that was
already accepted must not create a duplicate effect; the collector may answer
with the previously recorded acknowledgement. The relay must treat that
acknowledgement as valid only if it matches both `batchId` and `digest`
(`delivery.md` §3).
