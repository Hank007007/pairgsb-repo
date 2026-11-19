# signward agent policy

Status: normative. This document is the agreed contract between the platform
team and the owners of signward. Where the code and this document disagree,
this document is correct.

## 1. Scope

signward runs on a build machine. Build steps that need an Ed25519 signature
hand a `(keyHandle, purpose, payload)` triple to the agent and get a
signature back. The private key is loaded into the agent process; key
material must not leave it.

## 2. Key lifecycle

- Every key is created with an identifier, a policy (§3) and a creation
  instant.
- The agent allocates an opaque handle for each key. Handles are the only
  thing build steps see.
- A key can be destroyed explicitly. A destroyed key is retired for good:
  its handle must not yield a signature afterwards, and its on-disk seed
  file is removed.
- A key may carry a `NotAfter` instant. From that instant onward the key is
  no longer usable, and a resolve of its handle must fail. A key without a
  `NotAfter` does not expire on its own.
- Resolving a handle that is unknown, destroyed or past `NotAfter` is a
  refusal, not a nil result: callers get an error and no key. Each refusal
  is recorded in the audit trail (§7).

## 3. Per-key policy

Each key carries:

| Field | Meaning |
|---|---|
| `purposes` | Allow-list of purposes this key may sign for. |
| `maxSignaturesPerHour` | Cap on signatures issued per natural hour. |
| `maxPayloadBytes` | Cap on the size of a single payload, in bytes. |
| `notAfter` | End of the key's usable life. |

Policy defaults are permissive; an explicit value always applies.

## 4. Purpose matching

`purpose` is matched against the allow-list by exact string equality. A
purpose is allowed only if it is one of the listed purposes. There is no
prefix, substring or pattern matching: a list containing `sign` does not
allow `countersign` or `sign-raw`.

A request whose purpose is not allowed is refused, and nothing is signed.

## 5. Signature quota

`maxSignaturesPerHour` is counted over the natural hour (00:00–59:59) in the
agent's local time zone. The window rolls over at the top of each hour.

- The agent may issue at most `maxSignaturesPerHour` signatures in one
  window.
- Once the cap is reached, the next request in the same window is refused:
  the caller gets an error and no signature. Refusing after the signature
  has already been produced is not acceptable.
- A refused request does not consume quota.
- The counter is per key, not per session or per caller.
- Reaching the cap is recorded in the audit trail.

## 6. Payload limit and key exposure

- A payload larger than `maxPayloadBytes` is refused, and nothing is signed.
- Key material is never exported by any means. There is no configuration
  switch, environment variable, endpoint or diagnostic flag that makes the
  agent emit a private seed. Any attempt to ask for one is refused and
  recorded in the audit trail.

## 7. Audit trail

The agent appends one JSON object per line to its audit trail. At minimum the
following events are distinguishable: key creation, key load, key
destruction, a refused resolve, an issued signature, a refused signature
request, session unlock, session lock, session expiry and a refused export.
Each record carries the instant, the event, and where applicable the key
handle and purpose.

## 8. Sessions

A signing session is opened with `unlock` and closed with `lock`.

- Signing is only possible while a session is open. With no open session,
  `Sign` returns a locked-session error.
- A session is valid for **15 minutes** after the unlock. At or after
  `unlockedAt + 15m` the agent must behave as locked again: `Sign` returns
  the locked-session error and a session-expiry event is recorded.
- Opening a session again after it has expired restores signing for another
  15 minutes.
- `lock` ends a session immediately, whatever its remaining lifetime.
