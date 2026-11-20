# hostfold

`hostfold` is the authority-normalization layer that sits in front of an
outbound forward proxy. Upstream code hands it a URL or a bare authority; it
returns the canonical triple

```
(scheme, host_ascii, port)
```

that the policy layer — allow/deny lists loaded from `policies/egress.yaml` —
is allowed to look at.

Everything downstream is only as good as this step: if two spellings of the
same destination canonicalize differently, or a spelling resolves to a
different destination than the one the proxy dials, no policy can save you.

## Layout

```
hostfold/
  parse.py     authority splitting, canonicalize()
  idna.py      host -> ASCII mapping
  ip.py        IP literal classification and normalization
  policy.py    policy schema validation and first-match evaluation
  demo.py      prints a decision table for a set of sample targets
policies/
  egress.yaml  sample egress policy
docs/
  authority-normalization.md   the specification
tests/
```

## Usage

```python
from hostfold import canonicalize, load_policy, is_allowed

canonicalize("https://api.trusted.example/v1")
# Canonical(scheme='https', host='api.trusted.example', port=443)

policy = load_policy("policies/egress.yaml")
is_allowed("https://api.trusted.example/v1", policy)
# True
```

Refusals raise `hostfold.CanonicalizationError`; its `reason` attribute names
the rule that was violated. Policy documents are validated on load and raise
`hostfold.PolicyConfigError`.

## Demo

```
python -m hostfold.demo
```

## Tests

```
python -m pytest -q -p no:cacheprovider
```
