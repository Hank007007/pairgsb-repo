# scopebraid

`scopebraid` is the data platform's internal token exchange service. A
registered service presents an end-user access token and receives a narrower
access token scoped to a single downstream audience, in the style of OAuth 2.0
Token Exchange (RFC 8693).

## Layout

| Path | Purpose |
|---|---|
| `scopebraid/tokens.py` | compact token codec: encode, decode, verify |
| `scopebraid/exchange.py` | exchange request/response and the service entry point |
| `scopebraid/scopes.py` | scope parsing, narrowing, and coverage tests |
| `scopebraid/audience.py` | audience validation |
| `scopebraid/claims.py` | claim set and delegation-chain construction |
| `scopebraid/revoke.py` | `jti` revocation table and token validation |
| `scopebraid/registry.py` | client registrations |
| `docs/exchange-spec.md` | normative behaviour of the service |
| `docs/incident-reports.md` | post-incident report from a delegation incident |
| `tests/` | pytest suite |

## Usage

```python
from scopebraid.exchange import ExchangeRequest, TokenExchangeService
from scopebraid.registry import ClientRegistration, ClientRegistry

registry = ClientRegistry([
    ClientRegistration(
        client_id="svc-analytics",
        scopes=("datasets:read", "files:read"),
        audiences=("api://warehouse",),
    ),
])

service = TokenExchangeService(key=b"change-me", registry=registry)
result = service.exchange(
    ExchangeRequest(
        subject_token=subject_token,
        audience="api://warehouse",
        scope="datasets:read",
        client_id="svc-analytics",
    )
)
print(result.access_token, result.scope, result.expires_in)
```

Claims are carried in a compact three-segment token signed with HMAC-SHA256.
The signing key is the only shared secret; all validation is local.

## Tests

```
python -m pytest -q -p no:cacheprovider
```

Only the standard library and pytest are required.
