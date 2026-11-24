"""Audit trail.

Each cryptographic operation appends one JSON line to the audit sink.  The log
is meant for operators: it records who did what, not the data itself.
"""

from __future__ import annotations

import json
import threading
import time
from pathlib import Path


class AuditLog:
    def __init__(self, path: str | Path):
        self.path = Path(path)
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self._lock = threading.Lock()

    def record(self, entry: dict) -> None:
        """Append an already-sanitised audit entry for one request.

        The routing layer is responsible for omitting plaintext and request
        bodies (see ``docs/api.md`` §4.5).
        """
        line = json.dumps(entry, ensure_ascii=False, default=str)
        with self._lock:
            with self.path.open("a", encoding="utf-8") as fh:
                fh.write(line + "\n")


class NullAudit:
    def record(self, entry: dict) -> None:
        return None
