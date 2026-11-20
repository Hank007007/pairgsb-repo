# Verification

The gateway calls `verify(url, { keyring })` on every inbound presigned
request. Verification is all-or-nothing: the request proceeds only when the
signature and every bound field agree with the keyring.

## 1. Steps

1. Parse the URL. A URL that cannot be parsed is rejected.
2. Read the protocol parameters from the query: `X-Seal-Alg`,
   `X-Seal-Method`, `X-Seal-Expires`, `X-Seal-Tenant`, `X-Seal-Version`,
   `sig`. Any missing required parameter is a rejection.
3. Look up the tenant in the keyring. A tenant with no entry of its own is an
   error; there is no fallback namespace.
4. Derive `method`, `bucket`, `key` and the caller query from the request.
   Validate the key per `presign-format.md` §3 — an invalid key is rejected on
   this side too, regardless of what was signed.
5. Rebuild the canonical string exactly as `presign-format.md` §2 describes,
   using the method from `X-Seal-Method` and the expiry from
   `X-Seal-Expires`.
6. Recompute the HMAC for each acceptable key version and compare.
7. Check the expiry against the server clock.
8. Return the resolved request (`method`, `bucket`, `key`, `tenant`) so the
   gateway can authorize it.

## 2. Bound fields

Every field that appears in the canonical string is bound by the signature:

- Changing `X-Seal-Method` (for example replaying a `GET` link as `PUT`)
  changes the canonical string and must fail.
- Moving the path to another bucket or another key changes line 3 or line 4
  and must fail.
- Adding, removing, reordering or re-encoding caller query parameters must
  either be canonicalized to the same line (reordering, and equivalent
  encodings of the same bytes) or fail.
- Changing `X-Seal-Expires` changes line 6 and must fail.

`X-Seal-Version` is not signed; it is informational, and the verifier tries
the versions its keyring says are currently acceptable.

## 3. Expiry

`expires` is an absolute unix timestamp in seconds, decided by the issuer's
clock and compared against the verifier's clock only. The verifier never
trusts a client-supplied current time.

A link is acceptable while

```
now <= expires + CLOCK_SKEW
```

with `CLOCK_SKEW` = 30 seconds. At `now == expires + 30` the link is still
accepted; at `now == expires + 31` it is rejected. A link whose `expires` is
far in the future is accepted (the issuer chose the lifetime); a malformed or
non-numeric `expires` is a rejection.

## 4. Key rotation

A tenant may have several versions. The first entry in `keys.json` is the
active signing version; new links use it.

Older versions remain acceptable only until their `not_after` timestamp:

- A version without `not_after` is always acceptable.
- A version with `not_after` is acceptable while `now <= not_after`.
- A version that has passed `not_after` must not validate anything, even a
  signature that is otherwise correct.
- Two entries with the same `version` under one tenant are a configuration
  error and must be rejected when the keyring is loaded.

Verification order does not depend on how the file happens to be written.

## 5. Signature comparison

The recomputed signature and the presented `sig` are compared without an
early exit that depends on how many leading characters match: the comparison
time must not reveal the position of the first differing byte. Use a
constant-time comparison primitive on equal-length inputs.

## 6. Algorithm

`X-Seal-Alg` names the MAC algorithm. Only `HS256` is issued today, and it is
the only value the verifier accepts. An unknown or tampered `alg` value is a
rejection, never a downgrade to another digest or to an unsigned path.

## 7. Result shape

`verify` returns

```js
{ ok: true, tenant, method, bucket, key, expires, version }
```

on success and

```js
{ ok: false, reason }
```

on failure. `reason` is a short stable string for logging: `malformed-url`,
`missing-signature`, `unknown-tenant`, `invalid-key`, `expired`,
`bad-signature`, `unsupported-alg`.
