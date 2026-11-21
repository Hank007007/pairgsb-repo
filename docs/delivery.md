# Delivery: acknowledgement, retry, idempotency, concurrency

## §1 Sending

1. The relay sends one batch per request. Sending is an explicitly awaited
   operation; it resolves to a result that says whether the batch was accepted.
2. A send attempt has a bounded timeout. A timeout is a failure and is handled
   like any other transport failure.
3. The content sent is the batch produced by `batch-format.md` §1, with every
   record redacted per `redaction.md`.

## §2 Retry

1. A failed attempt is retried up to **3 attempts in total** for that batch, with
   a bounded backoff between attempts.
2. A retry sends the **same** batch: same `batchId`, same records in the same
   order, same `prevDigest`, same `digest`. Retries are not re-batched.
3. If the collector answers with a digest mismatch (`delivery.md` §3), that is
   not a transport failure: the batch was received but not accepted. It must be
   reported as a failure to the caller and must not advance the cursor. Whether
   it is retried with the same content is not required; reporting the failure
   is.
4. When all attempts fail, the send must report failure. A failed send must
   never be reported as success, and must never cause the cursor to advance.

## §3 Acknowledgement

1. The collector's response is a JSON object with `batchId` and `digest`.
2. A batch is considered accepted if and only if the response's `batchId`
   equals the sent batch's `batchId` **and** the response's `digest` equals the
   sent batch's `digest`.
3. An acknowledgement that fails any of these checks — a stale acknowledgement
   for a previous batch, a digest that does not match, a malformed response — is
   a failure. It does not advance the cursor. Acknowledging a batch must never
   be inferred from the mere fact that a response arrived.
4. Only after a valid acknowledgement may the cursor advance past the records in
   that batch, and only then does that batch's digest become the `prevDigest`
   for the next batch (`batch-format.md` §5).

## §4 Recovery and retransmission

1. After a failure, the batch's records remain in the spool in their original
   order and are retransmitted later — typically when connectivity returns.
2. Retransmission preserves order. The collector sees the same sequence of
   records it would have seen had the network not failed, and the `prevDigest`
   chain stays intact: the first batch after recovery links to the last
   acknowledged batch, which in turn links to its predecessor.
3. A batch that was accepted but whose acknowledgement was lost is retransmitted
   under the same `batchId`; the collector's idempotent reply is accepted per
   §3.

## §5 Concurrency

1. Delivery is serialized: at most one batch is in flight at a time. A batch is
   in flight from the moment it is sent until its acknowledgement is processed
   or its retries are exhausted.
2. A `flush()` call that arrives while a batch is in flight must not be silently
   dropped. It must either wait for the in-flight batch and then send whatever
   remains, or report that work is in progress. Work that was queued while a
   batch was in flight must eventually be sent.
3. Two flushes issued in the same tick must not cause the second flush's work to
   be forgotten after the first acknowledgement.

## §6 Error reporting

1. All transport and protocol failures are observable by the caller of `flush()`
   / `deliver()`. Failures are not swallowed, and a default return value must
   never stand in for a failed operation.
2. The result of a delivery attempt distinguishes at least: accepted, rejected
   (bad acknowledgement), and failed (no valid acknowledgement obtained).
