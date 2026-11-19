# Design notes

## Purpose

`logline-audit-encode` is the single place where services turn an audit
event into a line for the SIEM. Several services import it, so the encoded
bytes are a cross-service contract: the receiver parses them by the rules in
[`docs/log-encoding.md`](log-encoding.md), not by anything the encoder
happens to produce.

The repository is a working encoder whose output is checked against that
contract. The specification is the source of truth; this note explains how
the code is put together and what conformance means for it.

## Package layout

```
cmd/auditdemo    encodes one sample event; -format kv|json, -hostile
cmd/siemcheck    reads a record stream and counts the events it sees
internal/audit   the Event type handed to the encoder
internal/fields  field-set collection, value normalization, error values
internal/encode  Encoder, kv renderer, json renderer
docs/            the encoding specification
```

### `internal/audit`

`Event` carries the twelve core values plus `Extra`, a map of
request-derived attributes. Values are untrusted strings.

### `internal/fields`

`Set` collects fields in insertion order. Each field records its key, its
value, and the `Source` that supplied it. Name policy (the grammar, reserved
names, and duplicates) is enforced during collection, so that `Encode` fails
as a whole rather than skipping or rewriting a field; the errors declared in
this package report those violations. `Normalize` applies the value rules of
§4 of the specification.

The encoder never inspects `Extra` directly for policy; it hands every
extension field to collection and expects collection to fail on a violation.

### `internal/encode`

`Encoder` is bound to one format at construction. `Encode(event)` collects
the fields, then renders them. The result is the record only: no trailing
newline, no framing, no leading or trailing space.

Rendering walks the core fields in specification order and then the
extension fields in name order, so the output is deterministic for a given
event regardless of Go's map iteration order. `kv` and `json` share that
field ordering.

## Invariants the encoder must hold

These follow from the specification and are the properties a conforming
encoder is checked against:

1. **One record, one line.** For any input values, the returned bytes contain
   no LF, CR, U+2028, U+2029, or other C0 control byte literally.
2. **Round-trip.** Applying the receiver's reversal rules to a record
   recovers each normalized value exactly, and a JSON parser decodes each
   value to exactly the normalized value.
3. **Names are validated.** A field name outside the grammar in §3.3 is an
   error, in both formats.
4. **Reserved names are errors, not overrides.** An extension field that
   collides with a core field name fails collection.
5. **Duplicates are errors, not overwrites.** The second occurrence of a
   name fails collection, whatever its source.
6. **Truncation is UTF-8 safe.** A truncated value is valid UTF-8 and at most
   1024 bytes.
7. **Normalization is not lossy beyond the specification.** Valid UTF-8 is
   preserved byte for byte; only invalid subsequences are replaced and only
   values over the limit are truncated.
8. **The baseline is byte-stable.** The sample event in
   `internal/encode/testdata/golden/` must encode to the same bytes in both
   formats.

## Verification surfaces

- `go test ./...` runs the unit tests and the golden baseline.
- `go run ./cmd/auditdemo -format kv` and `-format json` print the sample
  record; diff against the golden files.
- `go run ./cmd/auditdemo -hostile | go run ./cmd/siemcheck` feeds a record
  whose user agent carries a CRLF sequence through the simulated receiver.
  A conforming encoder yields `events=1`.
- `go run ./cmd/siemcheck -keys` additionally prints the field names the
  receiver parsed, which shows whether a value introduced spurious fields.

## Testing policy

The committed tests cover the ordinary path: normalization of plain and
slightly unusual values, field-set behavior, and the golden baseline. New
cases that exercise the wire contract more closely belong in new files with
their own golden data. The baseline test and its golden files are frozen, so
ordinary-event output cannot drift as the edge of the contract is covered.

## Constraints

- Standard library only.
- `docs/log-encoding.md` is the contract and is not edited to match the
  implementation.
- `Encoder.Encode(event)` keeps its signature and return type.
- The baseline test and golden files are frozen.
