"""Decision values returned by the authorization resolver."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class Decision:
    """The outcome of evaluating one authorization question.

    ``allowed`` is the verdict. ``reasons`` carries short, stable markers that
    describe how the verdict was reached or why evaluation stopped early.
    """

    allowed: bool
    reasons: tuple[str, ...] = ()

    def __bool__(self) -> bool:
        return self.allowed
