# rowseal

Row-level security for the application database.

`rowseal` sits between application code and the driver. It reads the policy
documents under `policies/`, compiles the rules that apply to the current
subject into SQL boolean predicates, and splices those predicates into the
statements the application wants to run.

## Layout

| Path | Purpose |
|---|---|
| `rowseal/policy.py` | Loads and validates policy documents; resolves the rules for a table, command and subject |
| `rowseal/yamlmini.py` | Reader for the YAML subset used by policy documents |
| `rowseal/compile_.py` | Compiles condition expressions into predicate text and bound values |
| `rowseal/params.py` | Collects bound values and hands out placeholders |
| `rowseal/query.py` | Rewrites statements, adding predicates to the places that read or write rows |
| `rowseal/cache.py` | Caches compiled predicates |
| `policies/*.yaml` | Example policy documents |
| `docs/rls-design.md` | The behaviour this middleware is expected to have |
| `docs/incident-reports.md` | Post-incident write-ups |

## Policy documents

```yaml
version: 1

tables:
  orders:
    select:
      - name: regional-clerk
        roles: [clerk, manager]
        when: region = subject.region and status != 'void'
      - name: own-orders
        roles: [clerk]
        when: owner_id = subject.user_id
    update:
      - name: own-draft
        roles: [clerk]
        when: owner_id = subject.user_id and status = 'draft'

default:
  select: deny
  update: deny
  delete: deny
```

* `tables.<table>.<command>` is a list of rules for `select`, `update` or
  `delete`.
* Every rule has `roles`; `name` and `when` are optional.
* `when` is a condition over columns, `subject.<attribute>` values and
  literals. It supports `and`, `or`, `not`, `in` and the comparison operators
  `=`, `!=`, `<>`, `<`, `<=`, `>`, `>=`.
* `default` says what happens for commands on tables that have no entry.
  An entry is either `allow`, `deny`, or a list of rules.

## Using it

```python
from rowseal import Context, load_policy_set, configure, apply

configure(load_policy_set("policies/base.yaml"))

ctx = Context(role="clerk", attributes={"region": "eu", "user_id": 7})
statement = apply("select * from orders where id = 42", ctx)

driver.execute(statement.sql, statement.params)
```

`apply` returns a `Statement` with the rewritten SQL and the values to bind.
The same rewrite is available without the module-level policy set:

```python
from rowseal import compile_query

statement = compile_query(sql, policy_set, ctx)
```

## Development

```
python -m pip install pytest
python -m pytest -q -p no:cacheprovider
```

Only the standard library and pytest are required.
