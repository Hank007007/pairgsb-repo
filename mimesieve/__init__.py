"""Image intake gate for the upload gateway.

The gateway calls :func:`inspect` before writing an upload to object storage
and receives the detected container, its dimensions and an admission
verdict.  See ``docs/image-intake-policy.md`` for the rules enforced here.
"""

from __future__ import annotations

from .errors import IntakeError, Reason
from .model import Header, Result
from .verify import inspect

__all__ = ["IntakeError", "Reason", "Header", "Result", "inspect"]
