# rotgraph — design notes

This document describes the behaviour the orchestrator is meant to exhibit end to
end. It is a contract, not a description of any particular implementation. The
per-area rules are written out in `docs/rotation-plan.md`, `docs/swap-window.md`,
`docs/notify.md` and `docs/recovery.md`; this file ties them together and names
the invariants that must hold at every point.

## 1. The pipeline

One batch is one pass through this sequence:

```
read config
  → decide order            (or reject the batch, touching nothing)
  → for each key in that order:
        record intent to mint
        mint new key at vendor     (old key stays valid)
        record intent to push
        push new material to every consumer
        collect receipts, retrying where retries can help
        confirm both keys are usable
        wait until the dual-live window has elapsed
        record intent to retire
        retire the old key
  → report
```

Two escape hatches attach to that loop:

- **rollback** — taken when a step fails before the batch is done;
- **resume** — taken at process start when a journal for the batch already exists.

Both are described in §5 and §6.

## 2. Rejection happens before any effect

A batch is either accepted whole or refused whole. Refusal covers, at minimum:

- a duplicate key name;
- a dependency edge pointing at a name that is not in the batch;
- any cycle in the dependency graph, including a self-edge.

Refusal happens before the first vendor call and before the first journal write. A
refused batch therefore leaves the vendors untouched: no new key exists afterwards
that did not exist before.

## 3. Ordering

The order is a total order over the batch's keys with two properties:

1. it respects every dependency edge (dependency first);
2. among keys whose dependencies are already satisfied, it always takes the one
   that appears earliest in the config's `keys` array.

Property 2 is what makes two disjoint keys run in configuration order rather than
in name order, hash order, or in parallel. It is a requirement, not a tie-break
detail: the batch is a deterministic sequence.

Execution follows that order. It is not enough for a helper to be able to compute
the order; the run has to consume it, and the same rejection rules have to gate the
run.

## 4. Key lifecycle

Each key moves through a small state machine:

```
pending → created → notified → dwell → revoked
                  ↘        ↘
                   rolled_back   (batch failed)
```

- `created` means the vendor confirmed the new key; it is the start of the
  dual-live clock, and at that moment both keys are valid.
- `notified` means every consumer has accepted the new material.
- `dwell` means the dual-live window has been satisfied and retirement may start.
- `revoked` means the vendor has retired the old key.

Transitions are adjacent only. In particular:

- a key is never `revoked` before the new key exists and is usable;
- a key is never `revoked` while any consumer still has no receipt for it;
- a key is never `revoked` before `dwell` has elapsed since `created`.

The state a key is in is recoverable from the journal and is the main input to
restart logic.

## 5. Failure handling

### 5.1 Push failures

A consumer that answers with a client error has rejected the request; sending it
again cannot succeed. A consumer that fails with a server error, a connection
error or a read timeout may succeed on a later attempt, so pushes back off
exponentially, with a bounded number of attempts. Exhausting the attempts means
the consumer has no receipt.

The receipt-collection step reports, explicitly, which consumers have not
acknowledged. Callers decide whether to proceed based on that full answer, not on
whether *something* came back.

### 5.2 Retirement failures

If the vendor refuses to retire an old key, the batch has failed. The new key stays
— deleting it would leave the slot with nothing usable — and the old key is left in
whatever state the vendor reports.

### 5.3 Rollback

Rollback's job is to undo what this batch created: remove the new keys minted into
slots where the old key is still in service. The state after a clean rollback is
"exactly as before the batch".

A rolled-back batch is finished. Restarting it must not invent new external work.

## 6. Crash safety

The journal is the record of intent. Its rules:

1. **Intent first.** Every action that changes external state is written down before
   it is attempted. That ordering is what makes an interrupted step legible on
   restart: "we may have minted a key" is different from "we may not have".
2. **Durable and append-only.** Records are appended and flushed to stable storage
   before the action they describe begins. A record is either wholly present or
   wholly absent; a torn tail is discarded. Earlier records are never rewritten.
3. **No secrets.** Records carry references (vendor key ids, hashes), never key
   material. The journal is assumed to leak.
4. **Phase is load-bearing.** Restart logic distinguishes at least: no record at all;
   the new key exists but was never pushed; push and receipts are done but the
   window is not; the window is satisfied.

### 6.1 Resume

On restart, for every key in the batch:

- finished keys (`revoked`, `rolled_back`) are skipped entirely;
- a key whose last record says it was mid-step is reconciled against the vendor —
  the vendor is the authority on whether a key exists — and the batch continues from
  the reconciled state, reusing an already-minted key rather than minting a second
  one;
- the dual-live clock is not restarted; it is measured from the recorded creation
  time.

## 7. Verification

Before retiring an old key the orchestrator confirms the replacement is usable and
the old key still works. Verification is a live vendor call each time:

- both the new and the old key are checked;
- a timeout, connection failure or unparseable answer is a failure, not a pass;
- results are not reused across checks — a cached "yes" says nothing about the key
  now.

## 8. Webhook contract

- Three headers: `X-Rot-Timestamp` (current Unix seconds), `X-Rot-Nonce` (unique per
  request), `X-Rot-Signature`.
- Signature is HMAC-SHA256 over `timestamp|nonce|body`, hex encoded. The body is the
  exact byte string that is sent.
- The receiver rejects timestamps more than 300 seconds from its own clock.
- Because the timestamp and nonce are inside the signed string, a replayed request
  with different headers does not verify.
- Every retry is a fresh request: new timestamp, new nonce, matching signature.

## 9. Vendor adapters

An adapter hides one vendor's API behind `create`, `revoke`, `describe`, `probe`.

- `create` returns *this call's* key. Vendors that return a history of keys require
  the adapter to identify the one just minted — not to take a position in a list.
- `revoke` accepts the "must remain valid until" instant and passes it through.
  Vendors that treat an empty value as "immediately" are especially sensitive to
  this: an empty value silently disables the dual-live window.
- `describe` reports vendor truth; it does not consult a local cache.
- `probe` is a single liveness call with a given secret.

## 10. Invariants to check after a run

- A dependency's new key exists before its dependants are touched.
- Independent keys were processed in configuration order.
- A rejected batch created zero keys.
- While any key is in `created`/`notified`/`dwell`, the old key still works.
- No old key was retired without every receipt and the full window.
- The dual-live window is expressed to the vendor wherever the API can express it.
- Every external action has a journal record that predates it.
- Interrupting the process after a mint and restarting does not mint a second key.
- The journal contains no key material.
