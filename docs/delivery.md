# Delivery semantics

## 1. At-least-once

The agent delivers each record **at least once**. Exactly-once storage is the
collector's responsibility, achieved by deduplication keyed on the record's
identity (see §4). A record may therefore be transmitted many times; the
collector must store it once.

The agent's obligations:

* Do not advance the cursor past a batch until a 2xx response has been
  received for that batch.
* On any non-2xx response, network error, or timeout, retry the same batch.
* On restart, resume from the cursor and re-send anything not yet
  acknowledged.

The collector's obligations:

* Accept a batch, store each record once, and acknowledge the batch.
* Tolerate receiving a batch again — after an agent crash, a lost response, or
  an operator replay — without creating a second stored copy.

## 2. Acknowledgement

A `POST /batches` success is HTTP `200` with a JSON body:

```json
{
  "accepted": 3,
  "duplicates": 0,
  "origin": "web-01#a",
  "epoch": 3,
  "watermark": 56,
  "status": "origin web-01#a has no gaps"
}
```

* `accepted` — records newly stored by this request.
* `duplicates` — records recognised as already stored.
* `watermark` — the highest sequence number such that every sequence number
  from the stream's first record through it has been stored. It never moves
  backwards for a given `(origin, epoch)`.
* `status` — a human-readable description of what is still missing.

A non-2xx response is a rejection. The body must carry
`error.code`, `error.message`, and where applicable `error.field` and
`error.origin`. Rejections are not retried blindly by the agent: a `4xx`
indicates the batch is malformed or unauthorised and retrying it unchanged
will fail again; a `5xx` or transport failure is retried with backoff.

## 3. Cursor / accepted watermark relationship

The agent's cursor is "the next record the collector has *not* confirmed". The
collector's `watermark` is "the highest contiguous stored sequence number".
They need not be equal at every instant, but after a successful full drain of
a stream they must coincide at the last record. If an agent restarts and its
cursor lags the collector's watermark, the resend is harmless duplicates. If
the cursor leads the watermark, records are lost, so leading must never happen
as a result of a crash (see `spool-format.md` §5.2).

## 4. Deduplication identity

A stored record is identified by the triple `(origin, epoch, seq)`. Two
records with the same `seq` but different `origin` are different records and
must both be stored. Any cache or table used to detect duplicates must:

* be keyed on the full triple, and
* outlive the process: after a collector restart, a batch first seen before
  the restart must still be recognised as a duplicate.

The deduplication horizon must be at least as long as the maximum time an
agent may spend retrying. A replay of a batch that was stored before a
collector restart must still be recognised as a duplicate, so duplicate
detection state is part of what the collector must persist.

## 5. Out-of-order tolerance

Batches may arrive in any order. In particular a later batch may arrive before
an earlier one. The collector must:

* store every genuinely new record regardless of arrival order;
* not treat a record as stale merely because a record with a higher sequence
  number has already arrived;
* advance the `watermark` only over the contiguous prefix that has been
  stored;
* record missing regions explicitly, and **clear** a missing region once the
  records that fill it have been stored.

Example: sequence `10` arrives first, then `8`, then `9`, with `0..7` already
stored. After `10` the watermark stays at `7` and the missing region is
`8..9`. After `8` the region is `9..9`. After `9` the watermark is `10` and
the region list is empty.

## 6. Gap representation

Gaps are reported as **ranges**, not as a single flag, and are per
`(origin, epoch)`:

```json
{
  "origin": "web-01#a",
  "epoch": 3,
  "watermark": 7,
  "gaps": [{ "from": 8, "to": 9 }]
}
```

* `from` and `to` are inclusive sequence numbers.
* Ranges are maximal: adjacent missing sequence numbers are merged.
* A range is removed when it is filled, including when it is filled one record
  at a time.
* `GET /status` returns this structure for every tracked stream.
* A human-readable rendering must be actionable, e.g.
  `origin web-01#a missing seq 8..9`, so an operator can request exactly those
  records from the producer. A bare "has a gap" is not sufficient.

A missing region is opened only when the received sequence number is greater
than the current contiguous watermark + 1; it is never opened by an
out-of-order older record. An older record that was missing closes the
corresponding part of a region, because at-least-once delivery requires every
record to be stored even if a newer one was seen first.

## 7. Restart behaviour

* **Agent restart**: resume from the cursor; re-send the batch whose 2xx was
  not durably recorded; never skip unacknowledged records.
* **Collector restart**: deduplication state and per-stream gap state must
  reflect what is on disk. Replaying a batch accepted before the restart must
  yield `accepted: 0` (or a partial count covering only genuinely new records)
  and must not create a second stored copy. The reported watermark must not
  regress.

## 8. Failure classification

| condition | agent behaviour |
|---|---|
| 2xx | advance cursor durably, continue |
| 4xx (bad manifest, bad signature, count/digest mismatch) | report; do not silently advance |
| 5xx, timeout, connection error | retry with backoff |
| crash / kill | on restart, resume from last durable cursor |

The cursor must not be advanced for a batch until the collector has returned
2xx for that specific batch. A 4xx or 5xx leaves the cursor where it was, so
the next run offers the same records again.
