# grantshift

Internal OAuth 2 token service. Three grants, nothing else:

- **authorization code** — swap a one-time code for an access/refresh pair;
- **refresh** — rotate a refresh token for a new pair;
- **token exchange** (RFC 8693) — a service trades its token for a narrower
  one that lets it act on behalf of another principal.

Tokens are HS256 JWTs *and* rows in SQLite; the database is what makes
family-wide revocation possible.

## Layout

```
app/
  main.py        HTTP routes, error rendering
  grants.py      authorization endpoint + authorization-code grant
  refresh.py     refresh grant
  exchange.py    token exchange
  tokens.py      signing and persisting tokens
  revocation.py  revocation lookups
  auth.py        bearer verification shared by resource endpoints
  db.py          SQLite schema and queries
  jose_lite.py   HS256 JWT encode/decode (standard library only)
  scopes.py      scope normalisation
  clients.py     client registry
  schemas.py     request/response models
docs/
  token-service.md       normative behaviour per section (§1 – §9)
  integration-notes.md   caller-facing notes
tests/                   pytest suite, driven through the ASGI app
```

## Running the tests

```
pip install -e ".[dev]"
python -m pytest -q
```

The suite talks to the app over httpx's ASGI transport; no server needs to be
started. Each test gets a throwaway SQLite database under `tmp_path`.

## Running the service

```
pip install -e .
uvicorn app.asgi:app --port 8080
```

Environment variables: `GRANTSHIFT_SECRET`, `GRANTSHIFT_ISSUER`,
`GRANTSHIFT_DB`, `GRANTSHIFT_ACCESS_TTL`, `GRANTSHIFT_REFRESH_TTL`,
`GRANTSHIFT_CODE_TTL`.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| GET | `/healthz` | liveness |
| POST | `/authorize` | record a grant, return a code |
| POST | `/token` | authorization_code, refresh_token, token-exchange |
| POST | `/introspect` | RFC 7662 introspection |
| POST | `/revoke` | revoke a token and its family |
| GET | `/resource/me` | demo protected endpoint |

`docs/token-service.md` is the contract; `docs/integration-notes.md` collects
the mistakes callers make.
