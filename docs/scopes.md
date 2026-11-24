# Scope syntax, normalisation, aliases, and wildcards

This document specifies how scope strings are written, compared, and reduced.

## 1. Scope syntax

A scope collection is a set of scope tokens. On the wire it is a string of
scope tokens separated by whitespace, or a list of such strings. A scope token
contains no whitespace.

## 2. Normalisation

Normalisation turns any accepted spelling of a scope collection into one
canonical list:

- split on whitespace;
- drop empty entries;
- resolve aliases (see §3);
- remove duplicates;
- treat the collection as unordered; the canonical form is sorted.

Two scope collections are equal when their normalised forms are equal. For
example `"docs:write  docs:read"`, `"docs:read docs:write"`, and
`["read", "docs:write docs:read"]` all normalise to the same collection.

Normalisation is applied before any comparison: membership in a registered
scope set, membership in a subject grant, coverage checks, and the scope
written onto an issued token all operate on normalised values.

## 3. Aliases

Some scope names are aliases for a canonical name:

| Alias | Canonical |
|---|---|
| `read` | `docs:read` |

Alias resolution happens first, as part of normalisation. After resolution an
alias and its canonical name are the same scope: a client registered for
`read` and a subject token holding `docs:read` agree on that scope, and a
request for `read` against a grant of `docs:read` is satisfied.

## 4. Wildcards

### 4.1 Coverage

A wildcard scope of the form `<namespace>:*` stands for every scope with the
same `<namespace>` prefix, that is, every scope that starts with
`<namespace>:`.

`docs:*` covers `docs:read`, `docs:write`, and `docs:admin`.

### 4.2 The namespace itself

`docs:*` does not cover `docs`. A bare namespace is a distinct scope, not a
member of its own wildcard family. Coverage is directional and never inferred
in reverse: holding `docs:read` does not mean holding `docs:*`.

### 4.3 Requesting a wildcard

A request may ask for a wildcard. A requested wildcard is satisfied by the
concrete scopes in its family: the request selects exactly those family members
that are available from both the subject grant and the client registration, and
the issued token carries those concrete scopes. The wildcard is not written
onto the token in place of the scopes it selected, and it never selects the
bare namespace.

For example, a subject grant of `docs:read` together with a client registered
for `docs:read docs:write` satisfies a request for `docs:*` with `docs:read`.
The scope `docs` is outside the wildcard family and is never selected by
`docs:*`, and a client registered for `docs:read` alone cannot reach
`docs:write` by asking for `docs:*`.

### 4.4 Wildcards do not widen registrations

A client registered for `docs:read` cannot obtain `docs:write` by requesting
`docs:*`. Wildcard coverage never crosses a namespace, and `docs:*` never
covers `docs`.

## 5. Comparison summary

To decide whether a collection A covers a collection B:

- normalise both;
- every element of B must be present in A, where presence is judged after
  alias resolution and wildcard expansion as described above.

To compute an intersection of collections: normalise each, then keep the
elements of the requested collection that are covered by every other
collection. The result is reported in canonical form.
