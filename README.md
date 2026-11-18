# exprseal

A small, dependency-free expression engine for policy rules. Rules are written
by tenants and evaluated on each request; the engine lexes, parses, and walks
its own AST without ever calling `eval`.

```python
from exprseal import Context, evaluate, parse
from exprseal.context import Request, User

context = Context(
    User(
        id="u_1",
        plan="pro",
        role="admin",
        email="ops@example.com",
        age=34,
        verified=True,
        tenant_id="acme",
        groups=("engineering", "oncall"),
    ),
    Request(
        path="/api/v1/orders",
        method="GET",
        host="api.example.com",
        ip="203.0.113.9",
    ),
)

rule = parse('user.plan == "pro" and request.path.startswith("/api")')
assert evaluate(rule, context) is True
```

## Documentation

* `docs/expression-spec.md` — normative language and error contract
* `docs/field-map.md` — the complete list of readable paths
* `docs/design.md` — pipeline, isolation model, and intended behaviour

Where the code and `docs/expression-spec.md` disagree, the document wins.

## Development

```
python -m pytest -q -p no:cacheprovider
```

No third-party runtime dependencies; tests need only `pytest`.
