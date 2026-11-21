# outpostcall-egress

Outbound webhook delivery for the event platform: build the request target from
an integration's base URL and path template, assemble headers, sign the
request, send it, follow redirects, and retry transient failures.

## Install

```
pip install -e .
```

Standard library only; no runtime dependencies.

## Usage

```python
from outpostcall import deliver
from outpostcall.model import Event, Integration

integration = Integration(
    base_url="https://hooks.example.com/tenant-a",
    path_template="/v1/orders/{order_id}/events",
    secret=b"shared-secret",
    headers={"X-Tenant-ID": "t-42"},
    dynamic_headers={"X-Order": ["order_id"]},
    domains=("hooks.example.com",),
)

event = Event(fields={"order_id": "A1001"}, body=b'{"kind":"created"}')

result = deliver(integration, event, params={"page": "1"})
print(result.status_code)
```

Command line:

```
python -m outpostcall.cli \
  --base https://hooks.example.com \
  --template '/v1/orders/{order_id}/events' \
  --secret shared-secret \
  --field order_id=A1001 \
  --header X-Tenant-ID=t-42 \
  --domain hooks.example.com \
  --body '{"kind":"created"}'
```

`python -m outpostcall.cli --help` lists all options.

## Documentation

* `docs/urls.md` — URL and path construction
* `docs/query.md` — query component
* `docs/signature.md` — canonical string and signing
* `docs/egress.md` — allow-listing, redirects, retries
* `DESIGN.md` — how the pieces fit together

## Tests

```
python -m pytest -q -p no:cacheprovider
```

The suite uses a local recording HTTP server (`tests/conftest.py`) to inspect
the request line, query string, and headers that actually go out.