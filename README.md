# cfgfold

`cfgfold` is the control plane's tenant configuration loader. A tenant
uploads a YAML (or JSON) document from the console; the platform deep-merges
it into the built-in defaults and derives the quota, callback endpoints and
feature flags that take effect for that tenant.

The normative rules for reading, validating and merging a document live in
[`docs/config-rules.md`](docs/config-rules.md).

## Requirements

- Python 3.10+
- PyYAML 6.x
- pytest (tests only)

## Layout

```
cfgfold/
  defaults.py   platform default configuration
  errors.py     ConfigError and its stable rejection codes
  loader.py     parsing and structural validation of one document
  merge.py      deep merge of a tenant overlay into the defaults
  tenant.py     end-to-end entry point used by the control plane
  __main__.py   command line front end
docs/
  config-rules.md   normative behaviour, section by section
tests/
  test_loader.py    document parsing
  test_merge.py     deep merge
  test_tenant.py    end-to-end rendering
examples/
  tenant.yaml       a small tenant document
```

## Usage

As a library:

```python
from cfgfold import load_tenant_config

config = load_tenant_config(open("examples/tenant.yaml", encoding="utf-8").read())
config["quota"]["requests_per_minute"]   # 3000
config["quota"]["burst"]                 # 1200, still the platform default
```

From the command line:

```console
$ python -m cfgfold examples/tenant.yaml
{
  "callbacks": { ... },
  ...
}
```

Rejections are raised as `cfgfold.errors.ConfigError`. The `code` attribute
holds the stable reason described in §10 of the rules; the string form is
`cfgfold: <code>: <detail>`. The CLI prints that message to stderr and exits
with status 2.

## Tests

```console
$ python -m pytest -q -p no:cacheprovider
```
