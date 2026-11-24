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

    def record(self, request: dict, response: dict) -> None:
        """Append an audit entry for one completed request."""
        entry = {
            "ts": time.time(),
            "request": request,
            "response": response,
        }
        line = json.dumps(entry, ensure_ascii=False, default=str)
        with self._lock:
            with self.path.open("a", encoding="utf-8") as fh:
                fh.write(line + "\n")


class NullAudit:
    def record(self, request: dict, response: dict) -> None:
        return None
