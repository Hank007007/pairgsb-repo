# exchangefence

A small internal OAuth 2.0 token exchange service. Front-end services present a
user's subject token and receive a narrower access token that is safe to hand to
a downstream service. The same endpoint also handles refresh grants so that a
user whose permissions were reduced cannot recover the old permissions by
refreshing.

The wire shape follows RFC 8693:

```
POST /oauth/token
grant_type=urn:ietf:params:oauth:grant-type:token-exchange
subject_token=<token>
subject_token_type=urn:ietf:params:oauth:token-type:access_token
requested_scope=docs:read
actor_token=<token>          # optional
ttl=300                      # optional
```

Responses contain `access_token`, `issued_token_type`, `token_type`,
`expires_in`, `scope`, and `refresh_token`.

## Layout

```
exchangefence/
  exchange/service.py    token exchange and refresh orchestration
  scopes/canon.py        scope normalisation and coverage
  scopes/registry.py     client scope registration and aliases
  tokens/claims.py       access-token claim construction
  tokens/codec.py        compact signed token encoding
  grants/store.py        refresh grant storage
  http/handler.py        /oauth/token request handling
  providers.py           static client registrations
  policy.py              deployment policy (TTL cap, issuer, ...)
docs/
  exchange.md            endpoint contract and error codes
  scopes.md              scope syntax, aliases, wildcards
  delegation.md          delegation chains and downscoping
  grants.md              refresh grant lifecycle
  design.md              design notes for the service
tests/                   happy-path tests
```

## Running the tests

```
python -m pytest -q -p no:cacheprovider
```

Only the standard library and pytest are required.

## Tokens

Access tokens are compact signed strings produced by `exchangefence.tokens.codec`.
`codec.decode` verifies the signature before returning claims, so test tokens
can be produced with `codec.encode`.
