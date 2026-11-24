# Design

This document states what each stage of syslogfold is supposed to do. It
describes behaviour, not code. The normative rules live in
`docs/rfc5424-profile.md`, `docs/routing.md` and `docs/limits.md`; this file
explains how the stages fit together and what each one owns.

## The trust boundary

Everything to the left of the pipeline is untrusted: devices are operated by
other teams and applications copy user input into their log text. Everything to
the right is a SIEM that will split, index and alert on what we send. The layer
therefore owns one question on behalf of the collector: **how many fields does
this message have, and where does each one end.** A message that cannot be
answered unambiguously is rejected; it is never repaired into something that
looks plausible.

This is why the stages are ordered from strictest to most permissive: a field
boundary decided early cannot be re-litigated later, and a boundary that was
guessed in the header would silently shift every following field.

## Stage 1 — PRI and VERSION (`parse_pri`)

Input: the beginning of the wire text. Output: facility, severity, version, and
the remainder of the message.

- Recognise `<`, 1–3 decimal digits, `>`. Validate the digit count and the
  value range (0–191) and the no-leading-zero rule before deriving anything.
- Reject anything that is not a PRI. There is no default priority; a message
  without a PRI is not a syslog message.
- Accept exactly VERSION `1`. A missing version, `2`, or any longer token is a
  rejection.
- Facility and severity are derived from the validated value only.

## Stage 2 — header and body (`parse`)

Input: the remainder after the version. Output: timestamp, hostname,
app-name, procid, msgid, structured data, body.

- Fields are separated by single SP characters. Splitting on "whitespace"
  collapses runs and treats tabs as separators, and either would move a field
  boundary.
- The header has exactly five fields before structured data. A message that
  cannot supply all five is rejected. A message that has more text than the
  format allows before structured data must not have the excess folded into
  the body.
- Each header field is either NILVALUE or 1–255 printable US-ASCII characters.
  NILVALUE is represented as absence, not as the string `-`, so the two can be
  distinguished downstream.
- The timestamp is RFC 3339 with an explicit offset. It must be a real instant
  on the calendar, it must not be more than 300 s ahead of the receiver's
  clock, and fractional seconds must survive parsing.
- Structured data starts immediately after MSGID (one SP before it). Its end
  is determined by the structured-data grammar, not by searching for a
  character: a `]` inside a quoted parameter value is data, not a terminator.
  Everything after the final element — including a body that itself contains
  `[` or `]` — is body.
- The body may carry a UTF-8 BOM; the BOM is removed during decoding and is
  not part of the forwarded body.
- The 8192-byte limit is evaluated on the encoded size of the whole message.
  Non-ASCII text must be measured in bytes.

## Stage 3 — structured data (`parse_sd`, `escape_param`)

Input: the structured-data region (or NILVALUE). Output: a mapping of SD-ID to
parameters, plus the body that follows.

- An element is `[` SD-ID, then zero or more `SP PARAM-NAME "=" DQUOTE
  PARAM-VALUE DQUOTE`, then `]`.
- Scanning is stateful: inside a quoted value, `\` escapes the next character,
  so `\"`, `\\` and `\]` do not terminate the value or the element. A bare `"`
  or `]` inside a value is a malformed message, and the element terminator is
  the first `]` found outside a quoted value.
- PARAM-NAME is validated against its character set before it is used as a key.
  A name containing `=`, SP, `]` or `"` makes the message invalid rather than
  producing an odd key.
- `escape_param` is the inverse of the value parser: it escapes `"`, `\` and
  `]`, so that parsing its output returns the original value. It is used
  wherever this layer composes structured data or re-serialises one.
- SD-IDs are unique per message. A repeat is a rejection; the second element
  must not replace the first.
- Element and parameter counts are checked against `docs/limits.md` §2–§3.

## Stage 4 — routing (`tenant_for`)

Input: the sender address and the hostname. Output: the tenant.

- Build the route list once. For each route, test the sender address for
  containment in the route network using numeric address comparison.
- Test the hostname by comparing the first DNS label for equality with the
  route's label. Substring or prefix comparison lets a look-alike hostname
  (`acme-evil`) claim another tenant's channel.
- Validate the hostname before routing. A hostname that fails validation
  carries no routing information and the message cannot be attributed by
  hostname.
- Choose the most specific matching route by prefix length, independent of
  declaration order; break ties by declaration order.
- No match means `unassigned`. Silence is safer than a guess.

## Stage 5 — de-duplication (`dedup_key`)

Input: a parsed message. Output: a hashable key.

- The key identifies an event. Timestamp, hostname and msgid alone are not
  enough: two events can share all three in the same second. Severity (the low
  three bits of PRI) and PROCID must be part of the key so that distinct
  events are not dropped.
- The key is deterministic across processes: it is built from parsed values,
  not from object identity or insertion order.
- The pipeline keeps the caller-supplied `seen` set; a repeated key is
  reported as a duplicate, with the reason visible to the caller. It is not
  silently discarded.

## Stage 6 — CEF forwarding (`emit_cef`, `ingest`)

Input: parsed message, tenant, source. Output: one CEF line.

- The header has six `|`-separated fields after `CEF:0` (vendor, product,
  version, event class id, name, severity); `\` and `|` inside them are
  escaped.
- Extension values are escaped for `\`, `=`, LF and CR. The downstream
  collector splits the extension on unescaped spaces, then on the first
  unescaped `=`. A value that contains those characters unescaped can inject a
  key/value pair or a whole new line into the collector.
- Because the escape is applied to the *value*, the extension must be
  assembled from the parsed fields directly. Re-serialising a field that was
  parsed with a wrong boundary would export the error downstream, and no
  downstream stage re-checks field counts.
- The extension key order is fixed and documented; it is part of the contract
  with the collector.
- `ingest` is the only place that turns an exception into a record. A rejected
  message is reported with `ok=False` and a structured error (which field,
  why). It must not be emitted as an accepted CEF record that happens to carry
  the original bytes in a message field — that would make a malformed line
  indistinguishable from a real event, exactly the failure in
  `docs/incidents.md` (2026-01-14).

## Failure reporting

Every rejection names the stage field (`pri`, `version`, `timestamp`,
`hostname`, `sd`, `size`, `dedup`) and a short reason. Callers decide whether
to drop, quarantine or alert; the pipeline's job is to make the distinction
between "accepted", "duplicate" and "rejected" impossible to confuse.
