# scopevault-tokencache

An OAuth2 `client_credentials` token cache for internal service-to-service
calls.  Business code asks for a token with an issuer, a client identifier and
a scope set; the cache serves a still-valid token when it has one and
otherwise exchanges one at the issuer's token endpoint.

```python
from scopevault import TokenCache

cache = TokenCache()
token = cache.get("https://auth.internal", "billing-svc", "invoices:read")
```

## Documentation

- `docs/token-cache.md` — cache behaviour, keying, lifetime, concurrency,
  issuer response handling, revocation and the audit trail.
- `docs/scope-policy.md` — accepted scope input forms and the canonical form
  used for keys and comparisons.

## Layout

```
scopevault/
  cache.py         TokenCache: get / put / revoke
  scopes.py        scope canonicalization
  client.py        token endpoint client and response parsing
  singleflight.py  collapses concurrent work for one key
  audit.py         structured audit logging
  models.py        TokenEntry
  errors.py        exception types
tests/             pytest suite
```

## Development

Everything uses the standard library.  Run the suite with:

```
python -m pytest -q -p no:cacheprovider
```

To point the cache at a real issuer, construct it with a `TokenClient` backed
by the default `HttpTransport`:

```python
from scopevault import TokenCache, TokenClient
from scopevault.client import HttpTransport

cache = TokenCache(client=TokenClient(transport=HttpTransport(timeout=5.0)))
```
