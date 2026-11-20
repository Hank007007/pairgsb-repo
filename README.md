# refgate

`refgate` loads service configuration from a base document plus an optional
environment overlay. Field values may be secret references such as
`secret://kv/billing/api_key`, `secret://env/DB_PASSWORD` or
`secret://file/certs/tls.pem`; the loader resolves them through pluggable
providers, caches results for a short time, merges everything into a single
`Config`, and offers a redacted dump for logging.

The behaviour required of the library is specified in
[docs/secret-resolution.md](docs/secret-resolution.md).

## Layout

| Path | Purpose |
|---|---|
| `internal/config/load.go` | Reads the JSON documents, expands `${...}`, assembles the `Config` |
| `internal/config/merge.go` | Combines the base document with an environment overlay |
| `internal/config/ref.go` | Detects and resolves reference-valued fields |
| `internal/config/dump.go` | Redacted rendering for logs |
| `internal/resolve/registry.go` | Scheme dispatch and the resolver |
| `internal/resolve/kv.go` | `kv` provider backed by a JSON firmware file |
| `internal/resolve/env.go` | `env` provider |
| `internal/resolve/file.go` | `file` provider restricted to whitelisted directories |
| `internal/resolve/cache.go` | TTL cache in front of provider lookups |
| `testdata/` | Sample base/overlay documents, a kv firmware file and file secrets |

## Usage

```go
kv, err := resolve.NewKVProvider("testdata/kv.json")
if err != nil {
	return err
}

registry := resolve.NewRegistry()
registry.Register(kv)
registry.Register(resolve.NewEnvProvider())
registry.Register(resolve.NewFileProvider("/etc/refgate/secrets"))

cfg, err := config.Load(config.Options{
	BasePath:    "testdata/base.json",
	OverlayPath: "testdata/overlay.staging.json",
	Scope:       "staging",
	Vars:        map[string]string{"SERVICE_NAME": "checkout"},
	Resolver:    resolve.NewResolver(registry, resolve.NewCache(30*time.Second)),
})
if err != nil {
	return err
}

log.Printf("config: %v", cfg.Dump())
```

## Running

```sh
go test ./...
go run ./cmd/refgate -base testdata/base.json -scope staging
```

The command in `cmd/refgate` is a small demo that loads the shipped testdata
with the kv, env and file providers registered and prints the redacted dump.

## Test data

- `testdata/base.json` — base document.
- `testdata/overlay.staging.json` — staging overlay.
- `testdata/kv.json` — kv firmware with `default` and `staging` scopes.
- `testdata/secrets/` — files served by the `file` provider; only this
  directory is whitelisted in the demo command.
