# snisieve

snisieve is the front half of an egress proxy: before a connection is forwarded
upstream it looks at the client's first TLS flight, recovers the server name
from the ClientHello, and checks that name against an allow-list. Connections
whose name is not explicitly allowed are closed.

It does not decrypt anything and does not speak TLS. It parses untrusted bytes.

## Layout

| path | role |
|---|---|
| `docs/record-layer.md` | record framing, handshake reassembly, one-ClientHello rule |
| `docs/hello.md` | ClientHello and server_name parsing, validation |
| `docs/policy.md` | rule file, matching, failure handling, peek budget |
| `docs/design.md` | the pipeline as a whole, and what each stage owes the next |
| `internal/record` | `ReadRecords`, `Reassemble` |
| `internal/hello` | `ParseClientHello`, `ExtractSNI` |
| `internal/policy` | `LoadRules`, `Match` |
| `internal/proxy` | `Peek`: read the first flight, decide |
| `cmd/snisieved` | listener that wires `Peek` to a forward/close decision |
| `testdata` | a couple of captured ClientHellos and an allow-list |

## Build and test

```sh
go build ./...
go test ./...
```

## Run

```sh
go run ./cmd/snisieved -listen 127.0.0.1:8443 -upstream 127.0.0.1:443 -rules testdata/allow.txt
```

`-listen`, `-upstream` and `-rules` are the only flags. Exit status is 0 on a
clean shutdown, 1 for a runtime failure (listener or accept), and 2 for a usage
or configuration failure (bad flag, unreadable or invalid rule file).

## Pipeline

```
read first flight  ->  frame records  ->  reassemble handshake
       ->  parse ClientHello  ->  extract server_name  ->  match rules
```

Each stage may fail. A failure at any stage means the connection is not
forwarded; see `docs/policy.md` §4.

## Rule file

One rule per line, `#` comments and blank lines ignored:

```
example.com          exact match
*.example.net        exactly one extra label in front of example.net
```

The allow-list is the only source of permission; no name is allowed by default.
