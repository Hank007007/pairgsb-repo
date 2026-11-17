# gqladmit

`gqladmit` is the admission layer in front of a GraphQL gateway. A request
passes through it before anything is forwarded to the backend: the query text
is parsed into an AST, fragments are resolved, field-level authorisation and
introspection rules are applied, and the depth and field count of the query
are measured against the tenant's limits. Requests that fail any check are
refused; everything else is handed to the backend unchanged.

The backend serves every tenant from one schema, so a single query that slips
through can be expensive enough to hurt the database or broad enough to read
data the tenant was never granted.

## Layout

```
cmd/gqladmit      command line entry point
internal/graphql  document model
internal/parse    lexer, parser and fragment resolution
internal/cost     depth and field counting
internal/policy   introspection and field authorisation
internal/admit    the admission pipeline
docs              policy specification
```

## Policy

The normative description of what must be admitted and what must be rejected
is in [docs/query-policy.md](docs/query-policy.md).

## Usage

```sh
go run ./cmd/gqladmit -tenant acme <<'EOF'
query Dashboard {
  viewer {
    id
    posts(first: 10) { edges { node { title } } }
  }
}
EOF
```

```
admitted: query Dashboard depth=5 fields=6
```

Flags:

| Flag | Default | Meaning |
|---|---|---|
| `-tenant` | `public` | tenant the request is made for |
| `-max-depth` | `8` | maximum expanded depth |
| `-max-fields` | `200` | maximum expanded field count |
| `-restricted` | `userSecret,billingInfo` | comma separated restricted fields |
| `-granted` | empty | comma separated fields granted to the tenant |
| `-file` | empty | read the query from a file instead of stdin |

The command exits 0 when the query is admitted and 1 when it is rejected.

## Tests

```sh
go test ./...
```
