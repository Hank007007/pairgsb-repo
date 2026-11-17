# capbaton

Capability-token delegation chains for object storage.

An on-call engineer holds a token scoped to an incident bucket. They
attenuate it for a contractor, who attenuates it further for a
colleague. A downstream service receives the resulting `[]token.Link`
and must validate it before granting access.

```go
root, err := chain.IssueRoot(chain.Claim{
    TokenID: "tok-alice-root",
    Issuer:  "alice",
    Subject: "bob",
    Scopes:  []string{"s3:read:bucket/incident-42"},
    Caveats: []token.Caveat{{Name: chain.CaveatExpires, Value: "2029-12-31T23:59:59Z"}},
}, aliceKey)

child, err := chain.Narrow(root, chain.Claim{
    TokenID: "tok-bob-child",
    Issuer:  "bob",
    Subject: "carol",
    Scopes:  []string{"s3:read:bucket/incident-42/logs"},
    Caveats: root.Payload.Caveats,
}, bobKey)

err = chain.VerifyChain([]token.Link{root, child}, chain.Options{
    Keys:    keys,
    Revoked: revocations,
})
```

## Packages

| Package | Responsibility |
|---|---|
| `internal/token` | Wire representation of a link and its payload |
| `internal/scope` | Parsing `action:resource` scopes and containment |
| `internal/mac` | HMAC computation, verification, and key lookup |
| `internal/chain` | Root issuance, attenuation, chain verification |
| `internal/revoke` | Revoked token identifiers |

The normative behaviour is specified in [docs/delegation.md](docs/delegation.md).

## Development

```
go build ./...
go test ./...
```

Only the Go standard library is used.
