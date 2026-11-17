"""Append-only audit trail for elevation activity."""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from typing import Any, Iterator


@dataclass(frozen=True)
class AuditEntry:
    """One recorded event."""

    at: datetime
    event: str
    request_id: str
    actor: str
    detail: dict[str, Any] = field(default_factory=dict)


class AuditLog:
    """Collects audit entries in the order they occurred."""

    def __init__(self) -> None:
        self._entries: list[AuditEntry] = []

    def record(
        self,
        event: str,
        *,
        at: datetime,
        request_id: str,
        actor: str,
        **detail: Any,
    ) -> AuditEntry:
        entry = AuditEntry(
            at=at,
            event=event,
            request_id=request_id,
            actor=actor,
            detail=detail,
        )
        self._entries.append(entry)
        return entry

    @property
    def entries(self) -> tuple[AuditEntry, ...]:
        return tuple(self._entries)

    def for_request(self, request_id: str) -> list[AuditEntry]:
        return [entry for entry in self._entries if entry.request_id == request_id]

    def __iter__(self) -> Iterator[AuditEntry]:
        return iter(self._entries)

    def __len__(self) -> int:
        return len(self._entries)
