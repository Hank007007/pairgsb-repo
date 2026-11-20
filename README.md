# ptrloop

`ptrloop` is the intake layer of an internal DNS filtering resolver. It receives
the raw response bytes from an upstream recursive resolver, decodes them, decides
which answer records may enter the local cache, and hands the caller a message
that can be turned into a client response.

The upstream is untrusted. Everything in this repository is written against the
rules in [`docs/dns-intake.md`](docs/dns-intake.md); that document — not this
README — is the source of truth for what the code is supposed to do.

## Layout

```
cmd/dnsscan/        replays captured responses from testdata/ and prints the result
internal/wire/      header, name and record decoding (RFC 1035 / 3597)
internal/cache/     cache admission: bailiwick, owner normalisation, TTL ceiling
internal/limits/    per-message resource ceilings
docs/dns-intake.md  behavioural specification
testdata/           captured upstream responses, including deliberately hostile ones
tools/              generator for testdata/
```

## Building and running

```
go build ./...
go test ./...
go run ./cmd/dnsscan            # scans testdata/
go run ./cmd/dnsscan -dir DIR   # scans another directory of .bin captures
```

`dnsscan` prints, for each capture, the decoded header and question, the records
of each section, the screening decision, and the resource budget consumed by that
single parse.

## Regenerating the captures

```
python3 -I tools/gen_testdata.py
```

The generator only uses the Python standard library and writes every file listed
in §12 of the specification.

## Dependencies

Standard library only. The wire format is decoded by hand; the point of the
exercise is that decoder, so `miekg/dns` and `golang.org/x/net/dns/dnsmessage`
are deliberately not used.
