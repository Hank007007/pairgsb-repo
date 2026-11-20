"""Driver parameter handling for compiled predicates."""

from __future__ import annotations


class ParamList:
    """Collects bound values and hands out numbered placeholders.

    Placeholders use the ``$1``, ``$2``, ... style.  The list is kept in
    the same order as the placeholders so a driver can consume it directly.
    """

    def __init__(self, start: int = 1, values=None):
        self._next = start
        self._values = list(values) if values is not None else []

    def bind(self, value):
        """Bind ``value`` and return the placeholder that refers to it."""
        placeholder = "$%d" % self._next
        self._next += 1
        self._values.append(value)
        return placeholder

    def extend(self, values):
        """Bind each value in order and return the placeholders."""
        return [self.bind(value) for value in values]

    def values(self):
        """Return the bound values in placeholder order."""
        return list(self._values)

    @property
    def next_index(self) -> int:
        return self._next

    def __len__(self) -> int:
        return len(self._values)
