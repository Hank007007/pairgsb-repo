# confseal

confseal is the intake service for outbound-filter policy files. An operator
pastes a YAML policy document into the back office; confseal reads it,
compiles it into the policy object the filter engine loads, and hands that
object over for hot reload.

```
submitted text ──► intake ──► expand aliases ──► duplicate keys ──► compile ──► response
```

## Layout

| path | contents |
|---|---|
| `confseal/intake/loader.py` | `load_document`, `_check_duplicates` |
| `confseal/intake/anchors.py` | `expand`, `Budget` |
| `confseal/policy/compile.py` | `compile_policy`, `Policy`, `Rule` |
| `confseal/policy/errors.py` | `PolicyError` |
| `confseal/api.py` | `handle_upload` |
| `docs/` | the behavioural contract |
| `testdata/` | sample policy files |
| `tests/` | test suite |

## Documents

* [`docs/intake.md`](docs/intake.md) — reading, tags, duplicate keys, merge
  keys, aliases and the expansion limits.
* [`docs/policy.md`](docs/policy.md) — rule fields, rule identity, rule order
  and the compiled object.
* [`docs/api.md`](docs/api.md) — the upload response and its statuses.

## Usage

```python
from confseal import compile_policy, handle_upload, load_document

document = load_document(open("testdata/allow_team_a.yaml").read())
policy = compile_policy(document)

result = handle_upload("version: 1\nrules: []\n")
assert result["status"] == 200
```

## Tests

```sh
python -m pytest -q -p no:cacheprovider
```
