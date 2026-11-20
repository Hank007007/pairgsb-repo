"""Cache for compiled policy predicates.

Compiling a condition means parsing and rendering, which is cheap but
happens on every statement.  The cache keeps rendered predicate text so a
steady workload does not pay for it more than once per policy shape.
"""

from __future__ import annotations

from collections import OrderedDict
from dataclasses import dataclass

from .compile_ import compile_policy
from .params import ParamList
from .policy import Context, PolicySet, ResolvedPolicy, resolve_policy


@dataclass(frozen=True)
class Compiled:
    """A rendered predicate together with the values it needs bound."""

    sql: str
    params: tuple


class CompileCache:
    """Bounded cache of compiled predicates."""

    def __init__(self, capacity: int = 256):
        if capacity < 1:
            raise ValueError("capacity must be at least 1")
        self.capacity = capacity
        self._entries = OrderedDict()
        self.hits = 0
        self.misses = 0

    def key(self, table: str, command: str):
        """Return the cache key for a table and command pair."""
        return (table, command)

    def get(self, key):
        if key in self._entries:
            self.hits += 1
            entry = self._entries.pop(key)
            self._entries[key] = entry
            return entry
        self.misses += 1
        return None

    def put(self, key, value):
        if key in self._entries:
            self._entries.pop(key)
        self._entries[key] = value
        while len(self._entries) > self.capacity:
            self._entries.popitem(last=False)
        return value

    def clear(self):
        self._entries.clear()
        self.hits = 0
        self.misses = 0

    def __len__(self):
        return len(self._entries)


def compile_cached(
    cache: CompileCache,
    policy_set: PolicySet,
    table: str,
    command: str,
    ctx: Context,
    prefix=None,
):
    """Return the compiled predicate for one table and command.

    The rendered predicate is reused across calls with the same key.  When
    the cache misses, the policy is resolved for ``ctx`` and compiled with a
    fresh parameter list.
    """
    key = cache.key(table, command)
    cached = cache.get(key)
    if cached is not None:
        return cached

    resolved = resolve_policy(policy_set, table, command, ctx)
    params = ParamList()
    sql = compile_policy(resolved, ctx, params, prefix=prefix)
    return cache.put(key, Compiled(sql=sql, params=tuple(params.values())))
