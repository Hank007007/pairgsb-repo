"""Resource limits applied to a single render."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class Limits:
    """Upper bounds for one render call.

    ``max_loop_iterations`` is the total number of loop iterations a single
    render may execute.  ``max_output_bytes`` bounds the size of the rendered
    document.  ``max_render_seconds`` bounds the wall-clock time spent inside
    one render call.
    """

    max_loop_iterations: int = 10_000
    max_output_bytes: int = 1_048_576
    max_render_seconds: float = 2.0


DEFAULT_LIMITS = Limits()
