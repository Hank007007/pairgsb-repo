"""Append-only record of what the orchestrator is about to do.

See ``docs/recovery.md``.  Journals are JSON Lines: one complete JSON object
per line.
"""

from __future__ import annotations

import json
import os
import re
import time
from dataclasses import dataclass, field
from typing import Any


DONE_PHASES = ("revoked", "rolled_back")
PHASE_ORDER = ("pending", "created", "notified", "dwell", "revoked")
TERMINAL_PHASES = frozenset(DONE_PHASES)

_SENSITIVE_KEY = re.compile(
    r"(secret|token|password|passwd|authorization|api[_-]?key|private[_-]?key)",
    re.IGNORECASE,
)
_MATERIAL_KEYS = frozenset({"key", "new_key", "old_key", "material", "credentials"})


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

    def next_action(self, key: str) -> str | None:
        phase = self.phases.get(key, "pending")
        if phase in DONE_PHASES:
            return None
        return {
            "pending": "create",
            "created": "reconcile_create_then_notify",
            "notified": "wait_dwell_and_revoke",
            "dwell": "revoke",
        }.get(phase, "create")

    def _next_steps(self) -> dict[str, str | None]:
        return {name: self.next_action(name) for name in self.phases}

    @property
    def next_step(self) -> dict[str, str | None]:
        return self._next_steps()

    @property
    def next_steps(self) -> dict[str, str | None]:
        return self._next_steps()

    @property
    def states(self) -> dict[str, str]:
        return dict(self.phases)


def _is_sensitive_key(key: Any) -> bool:
    if not isinstance(key, str):
        return False
    lowered = key.lower()
    if _SENSITIVE_KEY.search(key):
        return True
    return lowered in _MATERIAL_KEYS


def redact(value: Any) -> Any:
    """Return a journal-safe copy with obvious credential material removed."""

    if isinstance(value, dict):
        safe: dict[str, Any] = {}
        for key, item in value.items():
            if _is_sensitive_key(key):
                continue
            safe[key] = redact(item)
        return safe
    if isinstance(value, list):
        return [redact(item) for item in value]
    if isinstance(value, tuple):
        return [redact(item) for item in value]
    return value


def _read_raw(path: str) -> tuple[list[dict[str, Any]], list[str]]:
    """Read valid records and diagnostics from a JSON Lines journal."""

    if not os.path.exists(path):
        return [], []

    try:
        with open(path, "r", encoding="utf-8") as handle:
            text = handle.read()
    except OSError as exc:
        return [], [f"journal {path!r} could not be read: {exc}"]

    if not text.strip():
        return [], []

    # Compatibility with the pre-JSON-Lines prototype, which stored one JSON
    # array.  New writes always use one object per line.
    if text.lstrip().startswith("["):
        try:
            loaded = json.loads(text)
        except ValueError as exc:
            return [], [f"journal array could not be parsed: {exc}"]
        if isinstance(loaded, list):
            return [item for item in loaded if isinstance(item, dict)], []
        return [], ["journal root is not a JSON array"]

    records: list[dict[str, Any]] = []
    diagnostics: list[str] = []
    lines = text.splitlines()
    for line_number, line in enumerate(lines, 1):
        if not line.strip():
            continue
        try:
            item = json.loads(line)
        except ValueError as exc:
            tail = line_number == len(lines)
            location = "torn tail record" if tail else f"invalid record on line {line_number}"
            diagnostics.append(f"{location} discarded: {exc}")
            continue
        if not isinstance(item, dict):
            diagnostics.append(f"line {line_number} is not a JSON object; discarded")
            continue
        records.append(item)

    return records, diagnostics


def _read_all(path: str) -> list[dict[str, Any]]:
    records, _ = _read_raw(path)
    return records


def _ensure_fresh_line(path: str) -> None:
    try:
        if os.path.exists(path) and os.path.getsize(path) > 0:
            with open(path, "rb") as handle:
                handle.seek(-1, os.SEEK_END)
                if handle.read(1) != b"\n":
                    with open(path, "a", encoding="utf-8", newline="\n") as out:
                        out.write("\n")
                        out.flush()
                        os.fsync(out.fileno())
    except OSError:
        # Reading the final byte is an optimisation; the append below still
        # preserves the existing prefix and will raise if storage is unwritable.
        pass


def append(path: str, record: dict[str, Any]) -> None:
    """Durably append one JSON object line to the journal at ``path``."""

    parent = os.path.dirname(os.path.abspath(path))
    os.makedirs(parent, exist_ok=True)

    safe_record = dict(record)
    if "detail" in safe_record:
        safe_record["detail"] = redact(safe_record.get("detail"))
    safe_record.setdefault("ts", time.time())

    _ensure_fresh_line(path)
    line = json.dumps(safe_record, sort_keys=True, separators=(",", ":"), ensure_ascii=True)
    with open(path, "a", encoding="utf-8", newline="\n") as handle:
        handle.write(line + "\n")
        handle.flush()
        os.fsync(handle.fileno())


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
        "detail": redact(dict(detail or {})),
        "ts": time.time(),
    }


def _phase_allows(current: str | None, new_phase: str) -> bool:
    if current is None:
        return True
    if current == new_phase:
        return True
    if current in TERMINAL_PHASES:
        return False
    if new_phase in TERMINAL_PHASES:
        return True
    try:
        return PHASE_ORDER.index(new_phase) >= PHASE_ORDER.index(current)
    except ValueError:
        return False


def resume(path: str, plan: Any | None = None) -> ResumeState:
    """Reconstruct a batch's state from its journal."""

    records, diagnostics = _read_raw(path)
    batch = None
    phases: dict[str, str] = {}
    done: list[str] = []

    for record in records:
        name = record.get("key")
        if record.get("batch"):
            batch = record["batch"]
        if not isinstance(name, str) or not name:
            diagnostics.append(f"record without a key name: {record!r}")
            continue

        phase = record.get("phase")
        if not isinstance(phase, str) or phase not in PHASE_ORDER and phase not in TERMINAL_PHASES:
            diagnostics.append(f"key {name!r} has unknown phase {phase!r}; ignored")
            continue

        current = phases.get(name)
        if not _phase_allows(current, phase):
            diagnostics.append(
                f"contradictory phase for {name!r}: {current!r} followed by {phase!r}; "
                f"keeping {current!r}"
            )
            if current in DONE_PHASES and name not in done:
                done.append(name)
            continue

        phases[name] = phase
        if phase in DONE_PHASES and name not in done:
            done.append(name)

    if plan is not None:
        for name in getattr(plan, "names", []) or []:
            phases.setdefault(name, "pending")

    return ResumeState(
        batch=batch,
        records=records,
        done=done,
        phases=phases,
        diagnostics=diagnostics,
    )
