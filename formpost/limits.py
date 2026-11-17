"""Resource bounds for one multipart submission.

The values here are part of the parsing contract published in
``docs/multipart-rules.md`` and are shared with the ingress configuration.
"""

from __future__ import annotations

# Maximum number of leaf parts across the whole submission, all nesting
# levels included.
MAX_PARTS = 32

# Maximum on-wire size of a single leaf part body, in bytes.
MAX_PART_BYTES = 1_000_000

# Maximum size of an entire request body, in bytes.
MAX_TOTAL_BYTES = 8_000_000

# Maximum nesting level.  Top-level parts are at depth 1.
MAX_DEPTH = 4

__all__ = [
    "MAX_PARTS",
    "MAX_PART_BYTES",
    "MAX_TOTAL_BYTES",
    "MAX_DEPTH",
]
