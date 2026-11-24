# Fixtures

`keys/` holds persisted key records used when exercising the service against a
stable key set.

| file | tenant | key name | versions |
|---|---|---|---|
| `acme-billing.json` | `acme` | `billing` | 1, 2 (latest 2) |
| `globex-billing.json` | `globex` | `billing` | 1 |

Both tenants deliberately own a key of the same name. Version material is fixed
so that runs are reproducible.

The records use the same JSON shape the service writes: `tenant`, `name`,
`min_decrypt_version`, `latest_version`, and a `versions` map from version
number to hex-encoded key material.
