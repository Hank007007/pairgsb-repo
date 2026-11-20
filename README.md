# termveil

`termveil` is the render filter that sits between the log search service and
the terminal. Matched log lines are joined into a `text/plain` response; this
package decides which bytes of that response are allowed to reach the terminal
and which must be shown as visible text.

Log content is untrusted. Applications write user input into their logs
verbatim, so a log line may contain escape sequences, OSC hyperlinks, C1
controls, bidirectional overrides, or zero-width characters. The filter applies
one of three policies:

| Policy | Behaviour |
|---|---|
| `plain` | every control sequence becomes visible text |
| `color` | only white-listed SGR sequences survive; everything else is escaped |
| `width` | filters like `color`, then truncates by display columns |

A policy name that is not one of the three above is an error.

The full contract lives in [`docs/render-policy.md`](docs/render-policy.md).

## Layout

```
internal/scan/scan.go     sequence scanning and rune decoding
internal/scan/width.go    display width and column truncation
internal/render/policy.go policy name resolution
internal/render/render.go policy application
cmd/preview               demo: renders testdata/*.log under every policy
testdata/                 sample log lines, including hostile ones
```

## Running

```
go test ./...
go run ./cmd/preview
go run ./cmd/preview -width 6
```

`cmd/preview` prints, for each sample log, the raw bytes and what each policy
produces from them. It is a demonstration program, not part of the filtering
path.
