"""Role inheritance graph."""

from __future__ import annotations

from collections import deque

from .model import Config


def closure(config: Config, role: str) -> tuple[str, ...]:
    """Return the roles that *role* inherits policy from."""
    found: list[str] = []
    seen = {role}
    queue: deque[str] = deque([role])
    while queue:
        current = queue.popleft()
        for parent in config.inherits.get(current, ()):
            if parent in seen or parent not in config.roles:
                continue
            seen.add(parent)
            found.append(parent)
            queue.append(parent)
    return tuple(found)


def route(config: Config, start: str, target: str) -> tuple[str, ...]:
    """Return a shortest inheritance chain from *start* to *target*.

    The chain includes both ends.  When ``start == target`` the chain is a
    single element.
    """
    if start == target:
        return (start,)
    queue: deque[tuple[str, ...]] = deque([(start,)])
    seen = {start}
    while queue:
        chain = queue.popleft()
        for parent in config.inherits.get(chain[-1], ()):
            if parent in seen:
                continue
            if parent == target:
                return chain + (parent,)
            seen.add(parent)
            queue.append(chain + (parent,))
    return (start, target)
