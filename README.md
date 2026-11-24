# rangecarve

rangecarve is the download edge that sits in front of an object store. Tenants
reach their media through it, so every request carries client-written headers:
players, downloaders and SDKs all send their own `Range`, `If-None-Match`,
`If-Match` and `If-Range`. The edge decides whether a reply is a full `200`, a
single or multipart `206`, a `304`, a `412` or a `416`, slices the bytes out of
the object it fetched from upstream, and keeps a short-lived copy of whole
objects so that repeated downloads do not go back to the store.

## Layout

| Path | Responsibility |
| --- | --- |
| `cmd/rangecarved` | process entry point, flags, the HTTP upstream client |
| `internal/rangehdr` | `Parse`: `Range` header → the intervals to serve |
| `internal/cond` | `Evaluate`: the conditional headers → one decision |
| `internal/respond` | `Build`: status code, response headers, multipart skeleton |
| `internal/edge` | `Serve`: cache, upstream, conditions, ranges and headers wired together |
| `internal/objcache` | `Key` / `Get` / `Put`: the in-process object cache |

## The specification

The rules the edge has to satisfy are written down in four documents. Each one
covers one layer and they are meant to be read together; a request travels
through all four.

- [`docs/range.md`](docs/range.md) — syntax of `Range`, the segment limit,
  satisfiability and how endpoints are clamped.
- [`docs/conditional.md`](docs/conditional.md) — validator precedence, the four
  conditional headers, and how `If-Range` gates `Range`.
- [`docs/response.md`](docs/response.md) — status codes, `Content-Range`,
  `multipart/byteranges`, download headers and `Accept-Ranges`.
- [`docs/cache.md`](docs/cache.md) — what the cache stores, how a key is built,
  what may be written and what may be read.

[`docs/design.md`](docs/design.md) ties the four together and states the
contract each layer owes the next.

## Request path

```
client headers
     │
     ▼
  Serve ──► objcache (whole objects, keyed per tenant/path/validator)
     │            │
     │            └── miss ──► upstream store ──► cache
     ▼
  cond.Evaluate ──► 304 / 412 / drop Range / continue
     │
     ▼
  rangehdr.Parse ──► 200 / 206 single / 206 multipart / 416
     │
     ▼
  respond.Build ──► status, headers, part headers
     │
     ▼
  body bytes + Content-Length
```

## Running

```
go run ./cmd/rangecarved -addr :8080 -upstream http://127.0.0.1:9000
```

Flags: `-addr`, `-upstream`, `-cache-entries`, `-cache-ttl`.

The tenant is taken from the `X-Rangecarve-Tenant` request header. The object is
the request path, and the store is expected to return `ETag`, `Last-Modified`,
`Content-Type`, `Cache-Control` and, when it knows one, the upload name in
`X-Rangecarve-Filename`.

## Tests

```
go test ./...
```

`internal/edge/serve_test.go` drives `Serve` through `httptest` against an
in-process store. `testdata/` holds the sample objects and a list of header
lines used when probing the edge by hand.

## Dependencies

Standard library only.
