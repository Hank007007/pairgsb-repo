# Query rules

Status: normative. This document defines the behaviour `POST /query` must
have. `docs/field-map.md` is the authoritative field list; the rules below
refer to it.

Terminology: a *request value* is any piece of the JSON body sent by the
caller. A *statement* is one SQL statement as understood by the storage
engine.

## §1 Request shape

`POST /query` accepts a JSON object:

| key      | required | meaning                           |
| -------- | -------- | --------------------------------- |
| `filter` | no       | filter expression                 |
| `sort`   | no       | one sort key, or an array of them |
| `limit`  | no       | page size                         |
| `offset` | no       | number of rows to skip            |

A filter expression is one of:

- a condition: `{ "field": <string>, "op": <string>, "value": <any> }`
- a conjunction: `{ "and": [<filter>, ...] }`
- a disjunction: `{ "or": [<filter>, ...] }`

A sort key is `{ "field": <string>, "direction": "asc" | "desc" }`.

Unknown top-level keys are ignored. A missing `filter` matches every row. A
missing `sort` leaves the row order unspecified. Paging defaults are given in
§5.

A body that is not a JSON object, or a filter expression that matches none of
the shapes above, is rejected as described in §7 (`invalid_query`).

## §2 Values are bound, never written into SQL

Every request value used as a data value must reach the storage engine as a
bound placeholder. The SQL text must not contain the request value in any
form, including escaped or quoted forms. This covers strings, numbers,
timestamps, every element of an `in` list, and the search text of a `like`
condition.

The only text taken from the request that may appear in the SQL text is a
field name admitted by §3, and it may appear only as a quoted identifier.

## §3 Field names and sort fields

The field of a condition and the field of a sort key must each be a key of
`docs/field-map.md`. A field used in a sort key must additionally be marked
sortable in that table.

Any other name is rejected with error code `unknown_field`; the error message
must contain the offending name, and the offending name must also be carried
in the error's `field` property. A field name is never emitted as raw text: it
is emitted only as a quoted identifier, after it has been admitted.

## §4 `like`

`like` is a contains match over a text field. The request value is matched
literally:

- `%` in the value matches a percent sign, not a wildcard;
- `_` in the value matches an underscore, not a single character;
- `\` in the value matches a backslash.

The predicate declares `\` as its escape character. A value that is only
wildcard characters therefore matches only text that contains those
characters; it must not degrade into a match-everything filter.

## §5 Pagination

| parameter | default | accepted values                   |
| --------- | ------- | --------------------------------- |
| `limit`   | `50`    | integer, `1 <= limit <= 1000`     |
| `offset`  | `0`     | integer, `0 <= offset <= 1000000` |

`limit` and `offset` must be JSON numbers whose value is an integer. Numeric
strings (`"25"`), fractional numbers (`2.5`), negative numbers, values above
the upper bound, and any other JSON type are rejected with error code
`invalid_pagination`. The message must identify the offending parameter.

`offset` is allowed to be `0`, which skips no rows.

## §6 One statement per request

A single request executes exactly one statement. If the composed SQL text
would cause the storage engine to execute more than one statement, the
request is rejected with error code `multiple_statements` before anything is
executed. A single trailing semicolon followed only by whitespace is
permitted and ignored.

The service must guarantee this itself; it may not rely on the storage engine
or its driver to refuse the additional statements.

## §7 Errors

Rule violations are reported as `QueryError`:

| property | value                                                    |
| -------- | -------------------------------------------------------- |
| `name`   | `QueryError`                                             |
| `code`   | one of the codes below                                   |
| `field`  | the offending field name, when the rule is about a field |
| `status` | `400`                                                    |

`POST /query` answers with HTTP `400` and a body of the form:

```json
{ "error": { "code": "unknown_field", "message": "...", "field": "..." } }
```

`field` is omitted when the rule is not about a field. Codes:

| code                  | raised by                                     |
| --------------------- | --------------------------------------------- |
| `invalid_query`       | malformed body or malformed filter/sort shape |
| `unknown_field`       | §3                                            |
| `unknown_operator`    | `op` is not one of §8                         |
| `invalid_value`       | value of the wrong shape for the operator     |
| `invalid_sort`        | malformed sort key or direction               |
| `invalid_pagination`  | §5                                            |
| `multiple_statements` | §6                                            |

## §8 Operators

| op         | meaning               | value                                   |
| ---------- | --------------------- | --------------------------------------- |
| `eq`       | equal                 | one value; `null` matches `IS NULL`     |
| `ne`       | not equal             | one value; `null` matches `IS NOT NULL` |
| `lt`       | less than             | one value                               |
| `lte`      | less than or equal    | one value                               |
| `gt`       | greater than          | one value                               |
| `gte`      | greater than or equal | one value                               |
| `in`       | membership in a list  | non-empty array of values               |
| `like`     | contains match, see §4 | one string                             |
| `is_null`  | column is null        | none                                    |
| `not_null` | column is not null    | none                                    |

`in` requires a non-empty array; anything else is rejected with
`invalid_value`.
