"""Append-only record of what the orchestrator is about to do.

See ``docs/recovery.md``.
"""

from __future__ import annotations

import json
import os
import time
from dataclasses import dataclass, field
from typing import Any


DONE_PHASES = ("revoked", "rolled_back")


@dataclass
class ResumeState:
    """What :func:`resume` found in a journal."""

    batch: str | None
    records: list[dict[str, Any]] = field(default_factory=list)
    done: list[str] = field(default_factory=list)
    phases: dict[str, str] = field(default_factory=dict)
    diagnostics: list[str] = field(default_factory=list)

    @property
    def pending(self) -> list[str]:
        return [name for name in self.phases if name not in self.done]


def _read_all(path: str) -> list[dict[str, Any]]:
    if not os.path.exists(path):
        return []
    try:
        with open(path, "r", encoding="utf-8") as handle:
            loaded = json.load(handle)
    except (OSError, ValueError):
        return []
    if not isinstance(loaded, list):
        return []
    return [item for item in loaded if isinstance(item, dict)]


def append(path: str, record: dict[str, Any]) -> None:
    """Add ``record`` to the journal at ``path``."""
    record = dict(record)
    record.setdefault("ts", time.time())
    records = _read_all(path)
    records.append(record)
    with open(path, "w", encoding="utf-8") as handle:
        json.dump(records, handle)


def make_record(
    batch: str,
    key: str,
    action: str,
    phase: str,
    detail: dict[str, Any] | None = None,
) -> dict[str, Any]:
    """Build one journal record."""
    return {
        "batch": batch,
        "key": key,
        "action": action,
        "phase": phase,
        "detail": dict(detail or {}),
        "ts": time.time(),
    }


def resume(path: str) -> ResumeState:
    """Reconstruct a batch's state from its journal."""
    records = _read_all(path)
    batch = None
    phases: dict[str, str] = {}
    done: list[str] = []
    diagnostics: list[str] = []

    for record in records:
        name = record.get("key")
        if record.get("batch"):
            batch = record["batch"]
        if not isinstance(name, str):
            diagnostics.append(f"record without a key name: {record!r}")
            continue
        phase = record.get("phase")
        if isinstance(phase, str):
            phases[name] = phase
            if phase in DONE_PHASES and name not in done:
                done.append(name)

    return ResumeState(
        batch=batch,
        records=records,
        done=done,
        phases=phases,
        diagnostics=diagnostics,
    )
