# Redaction

Redaction happens on the sending side, after the batch is assembled and before
it is handed to `deliver`. It applies to every record, including gap records and
records that were retransmitted.

## §1 Scope

1. Redaction applies recursively to string values anywhere in a record: the
   `payload` (including nested objects and arrays), `meta` and other
   non-payload fields, and inside stringified log messages.
2. Secret detection is **case-insensitive** and must not require a word boundary
   before the key. It must find a key when it appears:
   - in a query-string or `key=value` form (`token=...`, `api_key=...`),
   - in a header form with a colon (`Authorization: Bearer ...`,
     `Set-Cookie: ...`, `Cookie: ...`),
   - as a JSON member name in either quoting style (`"token": "..."`,
     `"Token": "..."`), including when embedded in a stringified message.
3. Recognised secret keys include at least: `token`, `password`, `passwd`,
   `secret`, `api_key`, `apikey`, `access_key`, `cookie`, `set-cookie`,
   `authorization`, `auth`, `credential`, `session`, `sessionid`.
4. Bearer and other authentication-scheme credentials are secrets regardless of
   the key they appear under: `Bearer <credential>`, `Basic <credential>`, and
   `Token <credential>` forms must be removed wherever they appear.
5. In structured data (`meta` and other objects), a secret key's **value** is the
   secret. The value is replaced; the key may be retained so that consumers can
   tell that a field was redacted.

## §2 Replacement

1. The replacement marker is the fixed string `[REDACTED]`.
2. Redaction is applied to the value that is actually serialised for
   transmission. The shipped record must contain the marker where the secret
   was; the original secret must not appear anywhere in the outgoing request
   body, headers, or URLs.
3. Redaction does not mutate the record held by the spool if doing so would lose
   the original for a later retry; if the spool record is mutated, the mutation
   must still be safe to retransmit and must still produce the same digest.
4. Redaction is idempotent: applying it twice yields the same output as applying
   it once.

## §3 Verification

A test of redaction is not "the replacement function was called". It is: build
the outgoing payload, serialise it exactly as it will be transmitted, and assert
that the original secret strings do not occur. This must hold for secrets in
`message`, in nested `meta`, and in values reached through arrays.
