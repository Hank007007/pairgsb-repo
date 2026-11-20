# cellguard

Batch import for the sales operations console. `cellguard` decodes an uploaded
CSV or TSV spreadsheet export into typed rows for the bulk order endpoint, and
formats cells that are written back out for a spreadsheet to open.

## The contract

`docs/csv-intake.md` is the specification: record decoding, dialect selection,
header rules, coercion, limits, and export-cell escaping. `bin/demo.js` runs the
intake layer over the files in `fixtures/` and prints what it decodes.

## Layout

| path | role |
|---|---|
| `src/tokenizer.js` | `nextRecord`: turns decoded text into records of fields |
| `src/dialect.js` | `sniff`: chooses the delimiter from the declared and content signals |
| `src/cells.js` | `sanitizeCell` and `coerce`: typed intake values, safe export cells |
| `src/limits.js` | `check`: request, cell, field-count and record limits |
| `src/index.js` | `parseSheet`: the entry point that wires the above together |
| `bin/demo.js` | runs the intake layer over `fixtures/` |
| `fixtures/` | sample exports taken from the sales workflow |
| `test/` | `node:test` suites |

## Running

```sh
node bin/demo.js
node --test
```

No dependencies are required beyond Node.js 20 or newer.

## Conventions

- Standard library only. Do not add a CSV parser dependency; the record decoder
  is intentionally hand-written.
- `parseSheet` is a single synchronous pass. Keep it that way; do not turn it
  into a streaming pipeline.
- Anything the caller builds orders from must be complete or absent. A result
  carrying an error never carries rows (see `docs/csv-intake.md` §1.3).
