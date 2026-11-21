# Design

This document walks a request through invitefence from the front-end payload
to the bytes handed to the mail transport. It states the behaviour each stage
is responsible for and the invariants that must hold across stage boundaries.
It deliberately describes *what* must be true, not *how* to compute it; the
normative detail lives in `docs/ical.md`, `docs/params.md`, `docs/recur.md`
and `docs/mime.md`.

## 1. The pipeline

```
request JSON
  │
  ├─ normalise fields          (src/http/invite.js)
  │
  ├─ validate + expand RRULE   (src/ics/recur.js)
  │     └─ occurrence count → busy minutes
  │
  ├─ escape TEXT values        (src/ics/text.js)
  │     └─ render parameters   (src/ics/params.js)
  │
  ├─ assemble VEVENT lines     (src/ics/serialize.js)
  │     └─ fold each line      (src/ics/text.js)
  │
  └─ wrap in MIME              (src/mime/envelope.js)
```

Each arrow is a hard boundary: a value that has crossed it must already
satisfy the invariant of the stage that produced it, and downstream stages
must not need to re-interpret raw user input.

## 2. Stage responsibilities

### 2.1 Field normalisation and validation

- Every string that reaches the renderer is either a value the service
  controls (property names, fixed tokens) or user text that will be escaped.
- Structured values (`start`, `end`, `until`, `sequence`, `count`, `interval`)
  are parsed and range-checked before rendering; a value that cannot be
  interpreted is a request error, not something that gets stringified and sent.
- An attendee is a triple of display name, e-mail address and a single role.
  The e-mail is an address, not free text: an address that is not a
  syntactically valid mail address makes the request invalid.
- The organiser is the same shape as an attendee, without a role.
- Missing optional fields stay missing; present-but-empty fields stay present
  and empty. The two are distinguishable in the output (`ical.md` §3.3).

### 2.2 Recurrence and busy time

- The rule is parsed into a descriptor with validated parts before anything is
  rendered (`recur.md` §2).
- The same parsed descriptor feeds two consumers: the `RRULE` property and the
  busy-time calculation. Both must agree on how many occurrences the series
  has inside the request window; a request that yields one number for the
  invite and another for the busy time does not meet the contract, and is not
  an acceptable approximation.
- Expansion is bounded (`recur.md` §4). The bound applies at the point where
  occurrences are materialised, so a pathological rule is rejected in bounded
  time regardless of which consumer asks for the expansion. The bound is a
  single shared value, not a per-call-site constant.
- `UNTIL` is compared as an absolute instant in UTC. A local `UNTIL` is
  converted through the meeting time zone first; an unresolvable zone or an
  unconvertible value rejects the rule rather than falling back to a
  wall-clock string comparison.
- Busy minutes are derived from the occurrence set that the invite actually
  represents — not from a separately computed, differently bounded count.

### 2.3 Escaping

- TEXT values are escaped exactly as `ical.md` §3.1 specifies, once, at the
  point where a raw user string enters a property value.
- After escaping, a value contains no raw CR and no raw LF. This is the
  invariant that keeps a display name from terminating its property.
- Escaping is applied before parameters are quoted, because `CN` is a text
  value that is then subject to parameter quoting (`params.md` §2.4).
- Escaping must be idempotent-safe in the sense that it never re-escapes the
  backslashes it introduced itself.

### 2.4 Parameters

- Parameter values are quoted according to `params.md` §2.1 and escaped
  per §2.2 before the quotes go on.
- A multi-valued parameter is emitted as one occurrence per value; a comma is
  never a separator inside a single value (`params.md` §2.3).
- In this service each attendee has one role, so each `ATTENDEE` line carries
  one `ROLE` occurrence whose value may legitimately contain a comma.
- `CN` and `ALTREP` follow their specific rules in §2.4 and §2.5.
- The parameter list is attached to the property name with `;`, and the value
  is separated from the last parameter with a single `:`. Neither the name
  nor the value may absorb that separator.

### 2.5 Assembly and folding

- A property line is assembled in a fixed order: name, parameters, `:`,
  value (`ical.md` §3.6). Folding operates on the assembled logical line, so
  the fold points are measured from the start of the physical line including
  the property name and parameters — not from the start of the value.
- Every logical line is folded on UTF-8 character boundaries to at most 75
  octets per physical line, with a leading space on continuations
  (`ical.md` §3.5).
- Property order inside `VEVENT` is stable and independent of user input.
  `SUMMARY` reflects the meeting title exactly when supplied.

### 2.6 MIME envelope

- The boundary is generated by the service, contains only characters valid in
  a boundary, and is verified absent from every body part before use
  (`mime.md` §3.2). If it occurs, generation repeats with a fresh candidate.
- The boundary is written into the container `Content-Type` using the same
  parameter-quoting rule as any other parameter value.
- Headers, delimiters and body parts all use CRLF throughout. The `.ics`
  document already ends its lines with CRLF and must not be rewritten or
  followed by a bare LF.

## 3. Cross-stage invariants

These are the properties a correct pipeline guarantees end to end, and the
ones a test can assert against the final output:

1. **No injected property.** For any user-supplied text, the number of
   properties in the rendered `.ics` is the number the request logically
   describes. A display name containing CRLF does not create an `ATTENDEE`,
   an `ORGANIZER`, or any other line.
2. **Round-trip stability.** Parsing the rendered `.ics` yields the attendee
   names, titles, locations and descriptions that were supplied (modulo the
   escape/normalise rules of `ical.md` §3.1).
3. **Fold safety.** Every physical line is at most 75 octets, every
   continuation starts with one space, no UTF-8 sequence is split, and
   unfolding restores the logical line exactly.
4. **Parameter integrity.** A value containing `,`, `;`, `:`, `"` or `\`
   cannot change the shape of the parameter list: it stays one value of one
   parameter.
5. **Bounded expansion.** The number of occurrences the service computes is
   bounded by the shared limit, and the same limit governs the invite and the
   busy-time figure.
6. **Boundary safety.** The boundary never appears in the body, and all line
   endings in the MIME message are CRLF.
7. **Failure is explicit.** A request the service cannot render correctly —
   invalid address, contradictory or unsupported rule, unresolvable zone,
   expansion over the limit — is rejected with an error. The service never
   emits an invite it knows to be approximate.

## 4. Error model

Validation failures surface as exceptions from the request handler and as
`400` responses from the HTTP interface. The renderer itself assumes its
inputs have already been validated; it does not silently repair them. Where a
limit is reached during expansion, the failure must originate in the expansion
step so that every caller — HTTP, CLI, or a future batch job — observes the
same behaviour.