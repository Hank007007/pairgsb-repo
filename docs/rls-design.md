# RLS design notes

Status: normative. This document states the behaviour the middleware is
expected to have. It does not prescribe an implementation.

## 1. Terms

* **Subject** — the authenticated caller a statement is executed for. It has
  an optional role and a set of named attributes.
* **Policy document** — a YAML file declaring rules per table and command.
  Commands are `select`, `update` and `delete`.
* **Rule** — a list of roles plus an optional condition. A rule applies to a
  subject when one of its roles matches the subject's role.
* **Condition** — an expression over table columns, subject attributes and
  literals, with `and`, `or`, `not` and `in`.
* **Predicate** — the SQL boolean expression produced from the rules that
  apply to one table reference.
* **Reference** — one occurrence of a base table in a statement, identified
  by its table name and (when present) its alias.

## 2. Literals and parameter binding

§2.1 Every value that comes from policy text or from subject attributes is a
bound parameter. Predicate text must contain only quoted identifiers, SQL
keywords and operators, and placeholders.

§2.2 This applies to all literal kinds: strings, numbers, booleans, null, and
each element of an `in` list. No literal kind may be rendered into the
predicate text.

§2.3 Values are never escaped into text. A value containing a quote, a
semicolon, a comment marker, or any other SQL punctuation is data and must
not change the shape of the statement.

§2.4 Placeholders and parameter values are ordered consistently. When a
statement carries several predicates, the parameter sequence follows the
order in which the placeholders appear in the final statement text.

§2.5 A condition may reference a subject attribute that is not set. That is a
rejected statement, never a silently skipped rule.

## 3. Boolean grouping and precedence

§3.1 `AND` binds tighter than `OR`; `NOT` binds tighter than `AND`.

§3.2 The compiled predicate must mean the same thing as the condition as
written. Whenever an `OR` expression becomes an operand of an enclosing
`AND`, it is grouped explicitly.

§3.3 The operand of `NOT` is grouped explicitly unless it is atomic (a single
comparison or `in` test).

§3.4 Several rules applying to the same table and command combine as a
disjunction. Each rule's predicate is grouped so that its internal `AND`
cannot capture a neighbouring rule, and the combined predicate is grouped
again when it is appended to an existing `WHERE` clause or embedded in a
larger expression.

§3.5 `a AND b OR c` means `(a AND b) OR c`. `a OR b AND c` means
`a OR (b AND c)`.

## 4. Table coverage

§4.1 Every base-table reference that contributes rows to a statement is
filtered by the policy for its own table and command.

§4.2 Coverage is not limited to the outermost `FROM` item. It includes:

* every `JOIN` operand, including comma joins;
* subqueries in `FROM`;
* `IN` and `EXISTS` subqueries;
* common table expressions, at the base tables inside each CTE body;
* nested combinations of the above.

§4.3 A predicate attached to a reference qualifies its columns with that
reference's alias, or with the table name when there is no alias. Two
references to the same table under different aliases each get their own
predicate, resolved and compiled for that alias.

§4.4 A CTE name is not a base table. Policies are applied to the base tables
that the CTE body reads, not to the CTE name itself.

§4.5 A statement that reads rows from a table through any path is subject to
that table's `select` policy, even when the statement's leading keyword is
not `SELECT`.

## 5. `RETURNING`

§5.1 `UPDATE ... RETURNING` and `DELETE ... RETURNING` return rows to the
caller. The returned rows must satisfy the command's write policy and the
table's `select` policy.

§5.2 Both predicates are part of the same statement. A row that the subject
may write but may not read is not returned.

## 6. Undeclared tables and the `default` section

§6.1 A table with no entry in `tables` follows the `default` section for the
command in question.

§6.2 If the `default` section is absent, or present without an entry for the
command, the result is denial: no rows are read, updated or deleted.

§6.3 A `default` entry is either a decision (`allow` or `deny`) or a list of
rules; rules are matched against the subject exactly as table rules are.

§6.4 A table that is declared but has no rules for a command is denied that
command. It does not fall back to the `default` section.

§6.5 An empty rule list means denial, never unrestricted access.

## 7. Role matching

§7.1 A rule's roles match the subject's role by exact equality.

§7.2 A rule listing `analyst` does not apply to `analyst-admin`, and a rule
listing `analyst-admin` does not apply to `analyst`.

§7.3 When a rule lists several roles, matching any one of them is sufficient.

## 8. Compilation cache

§8.1 A cached compiled predicate may be reused only by an identical
compilation request. The cache key must capture everything that determines
the result.

§8.2 The key includes at least:

* the policy document revision (its content identity, e.g. modification time
  and size);
* the table and command;
* the reference alias the predicate is qualified with;
* every subject input that was resolved while compiling, including the role
  and each attribute value read.

§8.3 A compiled entry carries the parameters that belong with its predicate.
Reusing an entry reuses its parameter sequence as well.

§8.4 After a policy document changes, entries compiled from the previous
revision must not be served. Reloading the document and reissuing a statement
must reflect the new rules.

## 9. Failure behaviour

§9.1 A policy document that cannot be read or validated is a startup error.
The middleware does not run with a partially loaded document.

§9.2 A statement that cannot be rewritten faithfully is rejected. It is never
forwarded unfiltered.
