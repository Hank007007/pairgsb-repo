# Token format

Normative specification for the wire representation of tokengrit API tokens.
Anything in this document is a hard compatibility requirement.

## §1 Grammar

A token is exactly:

```
tg_<id8>_<secret>
```

* Literal `tg` followed by `_`.
* `id8` — the public identifier: exactly 8 lowercase hexadecimal characters
  (`[0-9a-f]{8}`), i.e. 4 random bytes rendered as hex.
* Literal `_`.
* `secret` — exactly 64 lowercase hexadecimal characters (`[0-9a-f]{64}`),
  i.e. 32 random bytes rendered as hex.

The full token is 76 characters. There is no padding, whitespace, quoting, or
alternative separator. The grammar is case-sensitive: uppercase hex is not a
token.

## §2 Parsing

A parsed token has two variable parts: the identifier (`id8`) and the secret.
Parsing either succeeds and yields both parts, or fails. Input that does not
match §1, including non-string values, is rejected as malformed; it is never
repaired, truncated, case-folded, or partially accepted. Nothing may be
inferred from where a malformed value diverges from the grammar.

## §3 Public verification outcomes

Verification reports exactly one outcome. The public `reason` is drawn from:

| reason | meaning |
|---|---|
| `ok` | the token is usable for the requested operation |
| `invalid_token` | the presented credentials are not valid |
| `revoked` | the credentials belonged to a valid token that has been revoked |
| `expired` | the credentials belonged to a valid token that is no longer current |
| `insufficient_scope` | the token is valid but lacks the required scope |

`invalid_token` **must** cover all of the following without distinction:

1. a value that does not match §1;
2. an `id8` that matches no stored token;
3. a well-formed `id8` whose secret does not match the stored secret for that
   identifier.

Callers must not be able to tell cases 1–3 apart from the returned reason, and
the three cases must not be distinguishable through any other field of the
public result (including error text, result metadata, or whether any optional
field is populated). The same rule applies to the result of rotation when it
reports a failed predecessor.

## §4 Internal failure categories

Although §3 collapses the presentation, the audit trail keeps the precise
cause. The internal category is one of `malformed`, `unknown_prefix`,
`bad_secret`, and is recorded with the failure event. It is operational data
and never leaves the service boundary.

## §5 Identifier semantics

`id8` is a public lookup handle, not a secret: it is safe to log, and it is
expected to appear in audit records. It carries no authentication value on
its own. The secret is the only authenticating component.

Identifiers are allocated from a random source. A newly issued token must not
reuse an identifier already present in the store. Historical or externally
imported rows may nevertheless contain the same identifier on more than one
record; resolution must therefore not assume that an identifier identifies a
single record (see `docs/design.md` §4).
