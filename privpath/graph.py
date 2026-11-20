"""Role inheritance graph."""

from __future__ import annotations

from collections import deque

from .model import Config


def closure(config: Config, role: str) -> tuple[str, ...]:
    """Return the roles that *role* inherits policy from."""
    found: list[str] = []
    _walk(config, role, found, 1)
    return tuple(found)


def _walk(config: Config, role: str, found: list[str], depth: int) -> None:
    for parent in config.inherits.get(role, ()):
        if parent not in config.roles:
            continue
        if depth == 1:
            found.append(parent)
        _walk(config, parent, found, depth + 1)


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
