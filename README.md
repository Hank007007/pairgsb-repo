# relgraph

A relation-based authorization evaluator for a document collaboration service.

Relation tuples such as

```
doc:1#owner@user:alice
doc:1#viewer@group:eng#member
doc:1#parent@folder:7
```

are stored by `relgraph.store.TupleStore`. `relgraph.resolver.Resolver` answers
`check(subject, relation, object)` questions by reading the rewrite rules in
`schema/*.json`; no combination logic is hard-coded in the evaluator.

## Layout

```
relgraph/           implementation
  resolver.py       evaluation of check questions
  expand.py         expansion of subject references
  store.py          tuple storage and JSON persistence
  schema.py         schema loading and validation
  cache.py          memoization support
  decision.py       Decision value object
schema/             relation schemas, one JSON document per object type
docs/               normative semantics
tests/              test suite
```

## Usage

```python
from relgraph import Resolver, TupleStore

store = TupleStore("data/tuples.json")
store.add_tuples([
    "doc:1#parent@folder:7",
    "folder:7#viewer@user:alice",
])

resolver = Resolver(store)
print(resolver.check("user:alice", "viewer", "doc:1").allowed)  # True
```

Command line:

```
python -m relgraph --store data/tuples.json add "doc:1#viewer@user:alice"
python -m relgraph --store data/tuples.json check user:alice viewer doc:1
python -m relgraph --store data/tuples.json list
```

## Semantics

See [docs/relation-model.md](docs/relation-model.md) for the normative
behaviour of every rewrite node, userset subjects, the depth bound, and write
visibility.

## Schema contents

`schema/doc.json`

| relation | node |
|---|---|
| `owner` | `direct` |
| `editor` | `direct` |
| `parent` | `direct` |
| `viewer` | `union` of `direct`, `computed_userset(editor)`, `tuple_to_userset(parent → viewer)` |
| `reviewer` | `intersection` of `direct`, `computed_userset(editor)` |
| `blocked` | `direct` |
| `viewer_not_blocked` | `exclusion` of `computed_userset(viewer)` minus `computed_userset(blocked)` |

`schema/folder.json`

| relation | node |
|---|---|
| `owner` | `direct` |
| `editor` | `direct` |
| `viewer` | `union` of `direct`, `computed_userset(editor)` |

`schema/group.json`

| relation | node |
|---|---|
| `member` | `direct` |
| `admin` | `union` of `direct`, `computed_userset(member)` |

## Tests

```
python -m pytest -q -p no:cacheprovider
```
