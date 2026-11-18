# filesentry

A small privileged broker for file operations. An unprivileged process sends
one request over a Unix domain socket; `filesentryd` executes it with its own
credentials and writes one response back. What a caller may do is described by
the access file in `etc/access.json`.

## Layout

```
cmd/filesentryd        the daemon
cmd/filesentryctl      a client that sends one request by hand
etc/access.json        example access file
internal/wire          socket message types and their codec
internal/acl           access file parsing, path patterns, rule selection
internal/broker        listener, peer identity, request execution
docs/broker-design.md  normative description of the intended behaviour
docs/broker-incidents.md  reports collected from operators and on-call
```

## Building

```
go build ./...
go test ./...
```

Both commands work on any platform; the daemon and the client only do
something useful where Unix domain sockets and numeric ownership exist.

## Running

```
sudo mkdir -p /run/filesentry /srv/tenants
sudo ./filesentryd -access etc/access.json -socket /run/filesentry/broker.sock
./filesentryctl -socket /run/filesentry/broker.sock -op list -path /srv/tenants/acme
./filesentryctl -socket /run/filesentry/broker.sock -op read -path /srv/tenants/acme/reports/q1.csv
```

`filesentryctl` also accepts `-op write -path <p> -data <text> -mode 0640`,
`-op chown -path <p> -uid <n> -gid <n>`, and `-json` to print the response
verbatim.

## Protocol

One JSON request and one JSON response per connection, each terminated by a
newline. The message layout is documented in `docs/broker-design.md` §3 and is
fixed.

## Access file

```json
{
  "rules": [
    {"uid": 1001, "paths": ["/srv/tenants/acme/**"], "ops": ["read", "list"]},
    {"uid": 1001, "paths": ["/srv/tenants/acme/private/**"], "ops": ["deny"]}
  ]
}
```

Rules are read once at startup. An unknown op, a relative pattern, or a
pattern containing `.` or `..` prevents the daemon from starting.

## Documentation

`docs/broker-design.md` is the normative description of what the broker is
supposed to do: trust model, path patterns, rule selection, resolution and
containment, operations, and reason codes. `docs/broker-incidents.md` collects
operator reports and is informational only.
