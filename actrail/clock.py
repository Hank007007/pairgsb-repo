"""Injectable clock.

Every component that needs the current time accepts a ``Clock``.  Production
code uses :class:`SystemClock`; tests and simulations use :class:`FixedClock`
so that expiry and conditional access can be exercised deterministically.
"""

from __future__ import annotations

import time
from typing import Protocol, runtime_checkable


@runtime_checkable
class Clock(Protocol):
    """Minimal clock interface used by actrail."""

    def now(self) -> int:
        """Return the current POSIX timestamp in whole seconds."""


class SystemClock:
    """Clock backed by the operating system."""

    def now(self) -> int:
        return int(time.time())


class FixedClock:
    """A manually advanced clock.

    ``FixedClock`` is mutable on purpose: advancing it is how a test moves a
    token towards expiry without sleeping.
    """

    def __init__(self, epoch: int = 1_700_000_000) -> None:
        self._epoch = int(epoch)

    def now(self) -> int:
        return self._epoch

    def advance(self, seconds: int) -> int:
        self._epoch += int(seconds)
        return self._epoch

    def set(self, epoch: int) -> int:
        self._epoch = int(epoch)
        return self._epoch
