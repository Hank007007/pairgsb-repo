# CSV / TSV intake and export-cell contract

This document defines what `cellguard` must do when it turns an uploaded
spreadsheet export into rows of typed cells, and when it turns cells back into
text that a spreadsheet program will reopen safely.

It is normative: the numbered clauses are the behaviour the service is expected
to exhibit. It deliberately describes outcomes, not code.

Terminology:

- **field** — one cell's raw text inside a record.
- **record** — one logical table row, which may span several physical lines.
- **physical line** — a run of characters terminated by `\n`, `\r\n` or `\r`.
- **cell** — a field after type coercion and, on the export path, after
  formula-neutralisation.

The intake pipeline is:

```
bytes -> BOM / separator hint -> dialect choice -> records -> header check
      -> field count and size checks -> coercion -> cell rows
```

The export pipeline is:

```
cells -> formula-neutralisation -> quoted text
```

---

## §1 Pipeline contract

1. **§1.1** `parseSheet(input, options)` accepts the raw request body (a
   `Buffer`, `Uint8Array` or string) and the filename or declared media type of
   the upload. It returns an object with at least `header` (array of column
   names), `rows` (array of row objects or arrays, one per data record) and
   `error`.
2. **§1.2** On a successful parse, `error` is `null` and `rows` contains every
   data record after the header.
3. **§1.3** On any failure that prevents a complete, trustworthy table,
   `rows` **must be empty**. A partially decoded table must never be returned
   alongside a failure: the caller builds sales orders from whatever it is
   given, so a half table is worse than no table.
4. **§1.4** A failure is reported as an error object carrying a stable `code`
   (a machine-readable string), a human-readable `message`, and the 1-based
   `row` and `column` (in characters) at which decoding stopped, whenever a
   position exists. Structural failures that have no position use `row: 0` /
   `column: 0`.
5. **§1.5** Error codes for conditions that make the upload unusable begin with
   `fatal` so that callers which only inspect the code prefix stop the batch.
   Conditions that are tolerated but noteworthy begin with `warn`. There is no
   third class.

## §2 Record decoding

A record decoder walks the input once and emits records. It must not split the
input on newlines and then repair the pieces; newline handling is part of the
decoding state.

### §2.1 Line endings

1. **§2.1.1** `\n`, `\r\n` and a lone `\r` all terminate a record.
2. **§2.1.2** A `\r\n` pair terminates exactly one record; the pair is not two
   terminators.
3. **§2.1.3** Inside a quoted field, line-ending characters are data. They are
   preserved byte-for-byte: a `\r\n` inside quotes remains `\r\n`; it is not
   normalised to `\n`.
4. **§2.1.4** A final record without a trailing terminator is still a record.
5. **§2.1.5** An empty physical line at end of input yields no extra record;
   an empty line in the middle of the input yields an empty record (zero
   fields or one empty field, consistently).

### §2.2 Delimiters and quoting

1. **§2.2.1** Fields are separated by the record delimiter chosen for the
   dialect (§3).
2. **§2.2.2** A field begins quoted when its first character is `"`. Inside a
   quoted field, the delimiter and line terminators are literal data.
3. **§2.2.3** Inside a quoted field, `""` represents one literal `"`. The
   decoded value contains a single quote, never two.
4. **§2.2.4** Backslash has no special meaning. `\"` inside a quoted field is
   the two characters `\` and `"`; the quote still closes the field under
   §2.2.3's doubling rule.
5. **§2.2.5** After the closing quote of a quoted field, only the delimiter, a
   record terminator, or end of input may follow. Any other character is a
   malformed record and stops decoding with a `fatal` error naming the record
   and character position. It must not be silently appended to the value.
6. **§2.2.6** A quoted field that reaches end of input without a closing quote
   is a malformed record. Decoding stops and reports a `fatal` error at the
   opening quote; the partial record is not returned.
7. **§2.2.7** A quote that appears in the middle of an unquoted field is
   ordinary data and does not start a quoted region.

### §2.3 Byte-order mark and separator hint

1. **§2.3.1** A leading UTF-8 BOM (`EF BB BF`) is removed before any other
   processing. It must not appear in the first header name, in any value, or
   in the dialect sample.
2. **§2.3.2** A first line matching `sep=<one character>` (case-insensitive,
   optional surrounding whitespace, terminated like any other record) is a
   spreadsheet hint. It is consumed and does not become a header or a data
   record. The character it names is a delimiter candidate.
3. **§2.3.3** `sep=` applies only when it is the first non-BOM line. A later
   occurrence is ordinary data.

## §3 Dialect selection

1. **§3.1** The candidate delimiters are `,`, `\t` and `;`.
2. **§3.2** Declared signals from the upload (`.csv` / `.tsv` extension,
   media type) and content signals (the `sep=` hint, the shape of the data)
   must be reconciled, not silently overridden.
3. **§3.3** When the declared signal and the content agree, the declared
   delimiter is used.
4. **§3.4** When they disagree, the delimiter is chosen by examining the
   structure of the first several records — the number of fields per record
   and how consistent that number is across records — not by counting
   delimiter characters across the whole file. Text inside quoted fields is
   not counted as delimiter occurrences. When the evidence is not decisive,
   the declared signal wins.
5. **§3.5** An explicit `sep=` hint outranks an extension-based guess.
6. **§3.6** If the input cannot be decoded as a table under any candidate
   (for example every candidate produces one field for every record while the
   content plainly has structure), report a `fatal` error rather than guessing
   silently.
7. **§3.7** A file with a non-empty header and zero data records is valid; it
   yields an empty `rows` and no error.

## §4 Header rules

1. **§4.1** The first record after BOM and `sep=` handling is the header.
2. **§4.2** Header names are trimmed of surrounding whitespace before
   comparison and storage.
3. **§4.3** Empty header names (after trimming) are rejected with a `fatal`
   error naming the column. They must not be passed through to callers, who
   address fields by header name.
4. **§4.4** Duplicate header names (after trimming, compared
   case-sensitively) are rejected with a `fatal` error naming both column
   positions. A later column must not silently overwrite an earlier one.
5. **§4.5** The header count fixes the expected field count for every data
   record unless §4.6 applies.
6. **§4.6** A data record with a different field count from the header is a
   malformed record and is rejected with a `fatal` error carrying its record
   number.

## §5 Cell coercion

1. **§5.1** Each header name may carry a declared type of `text`, `number` or
   `date`; untyped columns are `text`.
2. **§5.2** `text` columns keep the decoded string, including leading and
   trailing whitespace.
3. **§5.3** `number` columns accept plain decimal numbers, including a leading
   `+` or `-` sign and a decimal point. The result is a JavaScript number.
4. **§5.4** A value that does not parse as a number in a `number` column is a
   cell-level failure reported against that row and column. It must not be
   silently passed through as a string, and it must not be silently discarded.
5. **§5.5** `date` columns accept the ISO forms `YYYY-MM-DD` and
   `YYYY-MM-DDThh:mm:ss`. Anything else is a cell-level failure reported
   against that row and column.
6. **§5.6** Empty cells are `null` for `number` and `date` columns and the
   empty string for `text` columns.
7. **§5.7** Coercion never mutates the source text used for diagnostics.

## §6 Limits

Limits are checked before a record becomes part of `rows`.

1. **§6.1** **Request body** — at most 8 MiB (8 * 1024 * 1024 bytes). A larger
   body is rejected with a `fatal` error before decoding.
2. **§6.2** **Cell size** — a single decoded field may be at most 32 KiB
   (32768 bytes in UTF-8, or 32768 characters; the limit is applied to the
   decoded value). A record containing an oversized field is rejected with a
   `fatal` error naming its record and column.
3. **§6.3** **Fields per record** — at most 64 fields in one record. A record
   with more is rejected with a `fatal` error naming its record.
4. **§6.4** **Records** — at most 50000 data records (the header does not
   count). Exceeding the limit is a `fatal` error naming the first record that
   would exceed it.
5. **§6.5** When a limit is exceeded, `rows` is empty (§1.3). The offending
   record must not appear in `rows`.

## §7 Export-cell escaping

When cells are written back out for a spreadsheet to open, every cell passes
through neutralisation.

1. **§7.1** A cell is *formula-active* when, after removing leading
   whitespace and NUL characters, its first character is one of `=`, `+`,
   `-`, `@`, or is a tab or carriage return — unless the whole value is a
   genuine number or date (§7.5). Formula-active cells are neutralised by
   prefixing a single apostrophe (`'`).
2. **§7.2** The check runs on the value as it will be written, regardless of
   the column's declared type. A numeric or date column is not exempt: a
   string that ended up in such a column still goes through neutralisation.
3. **§7.3** A value that is a genuine number, such as `-12.5` or the phone
   number `+8613800138000` written without separators, is not neutralised,
   because a spreadsheet does not evaluate a bare number as a formula. A
   string that starts with a sign but is not a valid number, such as
   `+1 555 0100`, is neutralised.
4. **§7.4** Neutralisation must be effective after quoting: when the value is
   emitted as a quoted CSV field, embedded `"` characters are doubled per
   §2.2.3, and the apostrophe prefix precedes the doubled content.
5. **§7.5** A value that is a genuine number or date (per §5.3/§5.5) is
   written as that number/date and is not neutralised. A string that merely
   begins with a sign and then contains formula syntax (for example `-2+3`)
   is not a genuine number and is neutralised. Coercion must not turn such a
   string into a number in the first place (§5.4).
6. **§7.6** Neutralisation is idempotent for the purposes of a single export
   pass: a cell already carrying the protective prefix is not given a second
   one.

## §8 `parseSheet` result rules

1. **§8.1** The result shape is `{ header, rows, error }` as in §1.
2. **§8.2** All failures, whether raised by decoding, header validation,
   coercion or limits, are caught at the `parseSheet` boundary and converted
   to the error object of §1.4. A failure must not escape as an uncaught
   exception.
3. **§8.3** Every error code carries the position of the failure: `row` is the
   1-based physical or logical record number as defined by the failing check,
   `column` is the 1-based field index or character offset as defined by that
   check. Both fields are always present.
4. **§8.4** `header` is `null` when the header itself could not be validated,
   and the validated header array otherwise.
5. **§8.5** Callers may rely on: `error !== null` implies `rows.length === 0`.
