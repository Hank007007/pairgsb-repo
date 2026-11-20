"""Splice compiled predicates into application SQL.

The rewriter understands enough SQL to locate the table a statement reads
from or writes to.  It does not parse expressions; it only looks for the
keywords it needs and for the table reference that follows them.
"""

from __future__ import annotations

import re
from dataclasses import dataclass

from .cache import CompileCache, Compiled, compile_cached
from .policy import COMMANDS, Context, PolicySet

# Keywords that end a table reference when they appear at the top level.
_TABLE_LIST_END = {
    "where",
    "group",
    "order",
    "having",
    "limit",
    "offset",
    "union",
    "except",
    "intersect",
    "window",
    "returning",
    "on",
    "using",
    "join",
    "inner",
    "left",
    "right",
    "full",
    "cross",
    "natural",
    "set",
    "values",
    "fetch",
    "for",
}


@dataclass(frozen=True)
class Statement:
    """A rewritten statement and the values to bind with it."""

    sql: str
    params: tuple


def split_statements(sql: str):
    """Split a script on top-level semicolons, ignoring quoted text."""
    statements = []
    buffer = []
    quote = None
    index = 0
    while index < len(sql):
        char = sql[index]
        if quote is not None:
            buffer.append(char)
            if char == quote:
                if index + 1 < len(sql) and sql[index + 1] == quote:
                    buffer.append(sql[index + 1])
                    index += 2
                    continue
                quote = None
            index += 1
            continue
        if char in ("'", '"'):
            quote = char
            buffer.append(char)
            index += 1
            continue
        if char == ";":
            statements.append("".join(buffer).strip())
            buffer = []
            index += 1
            continue
        buffer.append(char)
        index += 1
    tail = "".join(buffer).strip()
    if tail:
        statements.append(tail)
    return [statement for statement in statements if statement]


def _classify(sql: str) -> str:
    match = re.match(r"\s*([A-Za-z]+)", sql)
    if match is None:
        return ""
    return match.group(1).lower()


def _find_keyword(sql: str, word: str, start: int = 0) -> int:
    """Return the position of a top-level keyword, or ``-1``."""
    index = start
    quote = None
    depth = 0
    length = len(word)
    lowered = sql.lower()
    while index < len(sql):
        char = sql[index]
        if quote is not None:
            if char == quote:
                quote = None
            index += 1
            continue
        if char in ("'", '"'):
            quote = char
            index += 1
            continue
        if char == "(":
            depth += 1
            index += 1
            continue
        if char == ")":
            depth -= 1
            index += 1
            continue
        if depth == 0 and lowered.startswith(word, index):
            before_ok = index == 0 or not (
                sql[index - 1].isalnum() or sql[index - 1] == "_"
            )
            after = index + length
            after_ok = after >= len(sql) or not (
                sql[after].isalnum() or sql[after] == "_"
            )
            if before_ok and after_ok:
                return index
        index += 1
    return -1


def _read_identifier(sql: str, index: int):
    """Read a bare identifier starting at ``index``."""
    match = re.match(r"\s*([A-Za-z_][A-Za-z0-9_]*)", sql[index:])
    if match is None:
        return None, index
    name = match.group(1)
    end = index + match.end()
    if name.lower() in _TABLE_LIST_END:
        return None, index
    return name, end


def _statement_command(sql: str) -> str | None:
    verb = _classify(sql)
    if verb in ("update", "delete"):
        return verb
    if verb not in ("select", "with"):
        return None
    return "select"


def _target_table(sql: str):
    """Return ``(table, alias)`` for the table a statement uses."""
    verb = _classify(sql)
    if verb == "update":
        match = re.match(r"\s*update\s+(?:only\s+)?", sql, re.IGNORECASE)
        if match is None:
            return None
        return _read_reference(sql, match.end())
    start = _find_keyword(sql, "from")
    if start < 0:
        return None
    return _read_reference(sql, start + len("from"))


def _read_reference(sql: str, index: int):
    """Read ``table`` plus an optional ``AS alias`` or bare alias."""
    name, end = _read_identifier(sql, index)
    if name is None:
        return None
    alias = None
    word, after = _read_identifier(sql, end)
    if word is not None:
        if word.lower() == "as":
            alias, _ = _read_identifier(sql, after)
        else:
            alias = word
    return name, alias


def _returning_at(sql: str):
    """Return the position of a top-level RETURNING keyword, or ``None``."""
    position = _find_keyword(sql, "returning")
    if position < 0:
        return None
    return position


def _add_where(statement: str, predicate: str) -> str:
    """Insert ``predicate`` into the WHERE clause of ``statement``."""
    returning_at = _returning_at(statement)
    head = statement if returning_at is None else statement[:returning_at].rstrip()
    tail = "" if returning_at is None else " " + statement[returning_at:]
    position = _find_keyword(head, "where")
    if position < 0:
        return "%s WHERE %s%s" % (head, predicate, tail)
    after_where = position + len("where")
    existing = head[after_where:].strip()
    if existing:
        combined = "%s AND (%s)" % (existing, predicate)
    else:
        combined = predicate
    return "%s %s%s" % (head[:after_where], combined, tail)


class Rewriter:
    """Applies policies to statements for one subject."""

    def __init__(
        self,
        policy_set: PolicySet,
        ctx: Context,
        cache: CompileCache | None = None,
    ):
        self.policy_set = policy_set
        self.ctx = ctx
        self.cache = cache if cache is not None else CompileCache()

    def _predicate(self, table: str, command: str, alias=None) -> Compiled:
        if command not in COMMANDS:
            raise ValueError("unknown command %r" % command)
        return compile_cached(
            self.cache,
            self.policy_set,
            table,
            command,
            self.ctx,
            prefix=alias or table,
        )
    def apply(self, sql: str) -> Statement:
        """Return the rewritten statement and its bound values."""
        params = []
        pieces = []
        for statement in split_statements(sql):
            rewritten, values = self._rewrite_one(statement)
            pieces.append(rewritten)
            params.extend(values)
        return Statement(sql="; ".join(pieces), params=tuple(params))

    def _rewrite_one(self, statement: str):
        command = _statement_command(statement)
        if command is None:
            return statement, ()
        reference = _target_table(statement)
        if reference is None:
            return statement, ()
        table, alias = reference
        compiled = self._predicate(table, command, alias)
        return _add_where(statement, compiled.sql), tuple(compiled.params)


def compile_query(
    sql: str,
    policy_set: PolicySet,
    ctx: Context,
    cache: CompileCache | None = None,
):
    """Compile ``sql`` for ``ctx`` and return the rewritten statement."""
    return Rewriter(policy_set, ctx, cache).apply(sql)


_policy_set: PolicySet | None = None
_cache: CompileCache | None = None


def configure(policy_set: PolicySet, cache: CompileCache | None = None):
    """Install the policy set used by :func:`apply`.

    The compile cache is kept here so it survives across statements.
    """
    global _policy_set, _cache
    _policy_set = policy_set
    _cache = cache if cache is not None else CompileCache()
    return _policy_set


def reset():
    """Forget the policy set and cache installed by :func:`configure`."""
    global _policy_set, _cache
    _policy_set = None
    _cache = None


def apply(sql: str, ctx: Context):
    """Rewrite ``sql`` for ``ctx``.

    Uses the policy set installed with :func:`configure`.  The subject is
    taken from ``ctx`` for every call.
    """
    if _policy_set is None:
        raise RuntimeError("no policy set is configured")
    return compile_query(sql, _policy_set, ctx, _cache)


__all__ = [
    "Rewriter",
    "Statement",
    "compile_query",
    "split_statements",
    "apply",
]
