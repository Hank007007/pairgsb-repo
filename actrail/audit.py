"""Audit trail for impersonated calls.

Every downstream decision is appended to an :class:`AuditStore`.  A record
captures who the token spoke for, the whole delegation chain behind that
identity, and the outcome of the decision so that a later investigation can
reconstruct the call.
"""

from __future__ import annotations

import json
import threading
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Mapping

from .clock import Clock, SystemClock
from .token import act_chain


@dataclass(frozen=True)
class AuditRecord:
    """One authorization decision."""

    at: int
    action: str
    resource: str
    subject: str
    actor: str | None
    act: list[str]
    allowed: bool
    reason: str = ""
    jti: str | None = None

    def to_dict(self) -> dict[str, Any]:
        return {
            "at": self.at,
            "action": self.action,
            "resource": self.resource,
            "subject": self.subject,
            "actor": self.actor,
            "act": list(self.act),
            "allowed": self.allowed,
            "reason": self.reason,
            "jti": self.jti,
        }

    def to_json(self) -> str:
        return json.dumps(self.to_dict(), sort_keys=True)


class AuditStore:
    """In-memory audit log with optional JSON-lines persistence."""

    def __init__(self, path: str | Path | None = None) -> None:
        self._records: list[AuditRecord] = []
        self._lock = threading.Lock()
        self._path = Path(path) if path is not None else None

    def append(self, record: AuditRecord) -> AuditRecord:
        with self._lock:
            self._records.append(record)
            if self._path is not None:
                self._path.parent.mkdir(parents=True, exist_ok=True)
                with self._path.open("a", encoding="utf-8") as handle:
                    handle.write(record.to_json() + "\n")
        return record

    def records(self) -> tuple[AuditRecord, ...]:
        with self._lock:
            return tuple(self._records)

    def clear(self) -> None:
        with self._lock:
            self._records.clear()

    def __len__(self) -> int:
        with self._lock:
            return len(self._records)


_DEFAULT_STORE = AuditStore()


def default_store() -> AuditStore:
    """Return the process-wide audit store."""

    return _DEFAULT_STORE


def record(
    claims: Mapping[str, Any],
    *,
    action: str,
    resource: str,
    allowed: bool,
    reason: str = "",
    store: AuditStore | None = None,
    clock: Clock | None = None,
) -> AuditRecord:
    """Append an audit record describing a decision about ``claims``."""

    chain = act_chain(claims)
    actor = chain[-1] if chain else None
    act_value: list[str] = list(chain)
    audit_record = AuditRecord(
        at=(clock or SystemClock()).now(),
        action=action,
        resource=resource,
        subject=str(claims.get("sub", "")),
        actor=actor,
        act=act_value,
        allowed=bool(allowed),
        reason=reason,
        jti=claims.get("jti"),
    )
    (store if store is not None else default_store()).append(audit_record)
    return audit_record
