"""Verify a chain of sealed segments."""

from __future__ import annotations

import hashlib
import hmac
import json
from dataclasses import dataclass, field
from pathlib import Path

from . import keys as keys_mod
from .util import (
    RotsealError,
    entries_by_number,
    load_index,
    seal_filename,
    segment_filename,
    segments_dir,
)


@dataclass
class Report:
    """Outcome of a verification run."""

    archive: str = ""
    from_segment: int = 0
    checked: list = field(default_factory=list)
    failures: list = field(default_factory=list)
    warnings: list = field(default_factory=list)
    notes: list = field(default_factory=list)

    def add_checked(self, number, key_id, count, seq_start, seq_end):
        self.checked.append(
            {
                "segment": int(number),
                "key_id": key_id,
                "count": count,
                "seq_start": seq_start,
                "seq_end": seq_end,
            }
        )

    def add_failure(self, number, code, message, **extra):
        item = {
            "segment": None if number is None else int(number),
            "code": code,
            "message": message,
        }
        item.update(extra)
        self.failures.append(item)

    def add_warning(self, message, **extra):
        item = {"message": message}
        item.update(extra)
        self.warnings.append(item)

    def add_note(self, message, **extra):
        item = {"message": message}
        item.update(extra)
        self.notes.append(item)

    @property
    def ok(self) -> bool:
        return not self.failures

    @property
    def exit_code(self) -> int:
        return 0 if self.ok else 1

    def key_usage(self) -> dict:
        usage = {}
        for item in self.checked:
            usage[item["key_id"]] = usage.get(item["key_id"], 0) + 1
        return usage

    def as_dict(self) -> dict:
        return {
            "archive": self.archive,
            "from": self.from_segment,
            "ok": self.ok,
            "checked": self.checked,
            "failures": self.failures,
            "warnings": self.warnings,
            "notes": self.notes,
            "keys": self.key_usage(),
        }


def _signature_for(entry: dict, records_digest) -> str:
    secret = keys_mod.secret_bytes(entry)
    return hmac.new(
        secret, str(records_digest).encode("utf-8"), hashlib.sha256
    ).hexdigest()


def read_segment_lines(path) -> list:
    """Read the records of a segment file.

    Lines that do not decode are left out, so a segment can still be checked
    when its tail was written partially.
    """

    text = Path(path).read_text(encoding="utf-8", errors="ignore")
    lines = []
    for line in text.splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            lines.append(json.loads(line))
        except json.JSONDecodeError:
            continue
    return lines


def verify_chain(
    archive,
    keyring: dict,
    from_segment: int = 0,
    to_segment=None,
    now=None,
) -> Report:
    """Verify sealed segments from ``from_segment`` onwards.

    Blocks are visited in numeric order.  For each block the signature, the
    sidecar counters and the link to the preceding block are inspected; the
    findings are collected in the returned :class:`Report`.
    """

    directory = segments_dir(archive)
    report = Report(archive=str(archive), from_segment=int(from_segment))
    index = load_index(archive)
    entries = entries_by_number(index)

    if not entries:
        report.add_note("no sealed segments in archive")
        return report

    try:
        current_key = keys_mod.active_key(keyring)
    except RotsealError as exc:
        report.add_failure(None, "keyring", str(exc))
        return report

    numbers = sorted(entries)
    start = int(from_segment)
    previous_doc = None
    for number in numbers:
        if number < start:
            continue
        if to_segment is not None and number > int(to_segment):
            break

        segment = directory / segment_filename(number)
        sidecar = directory / seal_filename(number)

        if not segment.exists() or not sidecar.exists():
            report.add_warning(
                f"segment {number}: archive block is not present; skipped",
                segment=number,
            )
            continue

        try:
            doc = json.loads(sidecar.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as exc:
            report.add_failure(number, "sidecar", f"cannot read sidecar: {exc}")
            continue

        doc_key_id = str(doc.get("key_id"))
        if doc_key_id != str(current_key.get("id")):
            report.add_warning(
                f"segment {number}: sealed with key {doc_key_id}; "
                "the key material is not loaded",
                segment=number,
                key_id=doc_key_id,
            )
            continue

        expected_signature = _signature_for(current_key, doc.get("records_digest"))
        if not hmac.compare_digest(str(doc.get("signature", "")), expected_signature):
            report.add_failure(
                number,
                "signature",
                f"segment {number}: signature does not match the sidecar contents",
            )
            continue

        if previous_doc is not None:
            expected_prev_tip = previous_doc.get("tip")
            if doc.get("prev_tip") != expected_prev_tip:
                report.add_failure(
                    number,
                    "prev_tip",
                    f"segment {number}: prev_tip does not match the preceding block",
                    field="prev_tip",
                )

        try:
            lines = read_segment_lines(segment)
        except OSError as exc:
            report.add_failure(
                number, "records", f"segment {number}: unreadable records: {exc}"
            )
            continue

        report.add_checked(
            number,
            doc_key_id,
            len(lines),
            doc.get("seq_start"),
            doc.get("seq_end"),
        )
        previous_doc = doc

    if not report.checked and report.ok:
        report.add_note("nothing to verify in the requested range")
    return report
