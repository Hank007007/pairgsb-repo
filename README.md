# anchorveil

`anchorveil` is the YAML-subset loader used by the internal configuration
centre.  Deployment-unit alert rules and rate-limiting parameters are written
as manifests and loaded through this package before they are handed to the
deployment and rate-limiting components.

```python
from anchorveil import load_text, load_file

config = load_text(
    """
service: checkout
replicas: 3
limits: &limits
  rps: 200
  burst: 400
fallback: *limits
""",
    env={"REGION": "eu-west-1"},
)

settings = load_file("manifests/checkout.yaml")
```

## Supported manifest syntax

* mappings, sequences and scalars, nested by indentation;
* single- and double-quoted scalars;
* anchors (`&name`) and aliases (`*name`);
* the merge key `<<`;
* `!`-prefixed tags resolved through a tag registry.

The normative description — syntax, scalar typing, duplicate-key handling,
alias-expansion limits and tag-registry semantics — is
[`docs/manifest-format.md`](docs/manifest-format.md).  Background on why the
limits and the registry exist is in
[`docs/incident-reports.md`](docs/incident-reports.md).  The internal design
is described in [`DESIGN.md`](DESIGN.md).

## Public API

| Name | Purpose |
|---|---|
| `load_text(text, *, registry=None, env=None)` | load a manifest from a string |
| `load_file(path, *, registry=None, env=None)` | load a manifest from a file (`!include` is resolved relative to the file) |
| `TagRegistry` | the tag registry; `register(tag, handler)` adds a handler |
| `default_registry(env=None)` | the registry with `!env`, `!include`, `!secret` |
| `ManifestError` and subclasses | see §8 of the format document |

Both loaders return plain Python data: `dict`, `list`, `str`, `int`, `float`,
`bool` or `None`.

## Development

```bash
python -m pytest -q -p no:cacheprovider
```

Standard library only; `pytest` is the sole test dependency.
