"""Verify a chain of sealed segments."""

from __future__ import annotations

import hmac
import json
import re
from dataclasses import dataclass, field
from pathlib import Path

from . import keys as keys_mod
from .sealing import compute_digest, read_records, signature_for, tip_for
from .util import (
    RotsealError,
    entries_by_number,
    load_index,
    load_tombstones,
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


_SEGMENT_NAME = re.compile(r"^segment-(\d{6})\.(ndjson|seal)$")
_UNKNOWN = object()


def _scan_segments(directory: Path) -> dict:
    """Map block numbers to the segment/sidecar files that actually exist."""

    found = {}
    if not directory.exists():
        return found
    try:
        children = list(directory.iterdir())
    except OSError:
        return found
    for child in children:
        match = _SEGMENT_NAME.match(child.name)
        if not match:
            continue
        number = int(match.group(1))
        if number <= 0:
            continue
        found.setdefault(number, {})[match.group(2)] = child
    return found


def _read_sidecar(path: Path) -> dict:
    try:
        doc = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise RotsealError(f"cannot read sidecar {path}: {exc}") from exc
    if not isinstance(doc, dict):
        raise RotsealError(f"malformed sidecar: {path}")
    return doc


def _int_or_none(value):
    try:
        return int(value)
    except (TypeError, ValueError):
        return None


def read_segment_lines(path) -> list:
    """Compatibility wrapper around the strict segment reader."""

    return read_records(path)


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

    archive = Path(archive)
    directory = segments_dir(archive)
    report = Report(archive=str(archive), from_segment=int(from_segment))
    try:
        index = load_index(archive)
        entries = entries_by_number(index)
    except RotsealError as exc:
        report.add_failure(None, "index", str(exc))
        return report

    try:
        tombstone_list = load_tombstones(archive)
    except RotsealError as exc:
        report.add_failure(None, "tombstones", str(exc))
        tombstone_list = []
    tombstones = {}
    for item in tombstone_list:
        number = _int_or_none(item.get("number"))
        if number is not None:
            tombstones[number] = item

    disk = _scan_segments(directory)
    known_numbers = set(entries) | set(disk)

    if not known_numbers:
        if tombstones:
            report.add_note(
                "all %d sealed block(s) were removed under the retention policy"
                % len(tombstones)
            )
        else:
            report.add_note("no sealed segments in archive")
        return report

    highest = max(known_numbers)
    requested_start = int(from_segment)
    start = 1 if requested_start <= 0 else requested_start
    end = highest if to_segment is None else min(highest, int(to_segment))

    sidecars = {}
    tips = {}
    for number in sorted(set(disk) | set(entries)):
        path = disk.get(number, {}).get("seal")
        if path is None:
            continue
        try:
            doc = _read_sidecar(path)
        except RotsealError as exc:
            sidecars[number] = (None, str(exc))
            continue
        sidecars[number] = (doc, None)
        try:
            tips[number] = tip_for(number, doc)
        except (TypeError, ValueError, json.JSONDecodeError):
            pass

    measured_seq_end = {}
    declared_seq_end = {}
    for number, (doc, _error) in sidecars.items():
        if doc is not None:
            value = _int_or_none(doc.get("seq_end"))
            if value is not None:
                declared_seq_end[number] = value

    expected_archive_id = index.get("archive_id")
    if expected_archive_id is None:
        for number in sorted(tombstones):
            value = tombstones[number].get("archive_id")
            if value is not None:
                expected_archive_id = value
                break
    if expected_archive_id is None:
        for number in sorted(sidecars):
            doc = sidecars[number][0]
            if doc is not None and doc.get("archive_id") is not None:
                expected_archive_id = doc.get("archive_id")
                break

    for number in range(start, end + 1):
        info = disk.get(number, {})
        segment = info.get("ndjson")
        sidecar_path = info.get("seal")
        listed = number in entries

        if number in tombstones:
            if listed or segment is not None or sidecar_path is not None:
                report.add_failure(
                    number,
                    "tombstone",
                    f"segment {number}: block is tombstoned but still present",
                )
            continue

        if not listed and segment is None and sidecar_path is None:
            report.add_failure(
                number,
                "missing",
                f"segment {number}: archive block is missing",
            )
            continue

        if not listed:
            report.add_failure(
                number,
                "index",
                f"segment {number}: block is present but not listed in the index",
            )
            continue

        if segment is None:
            report.add_failure(
                number,
                "missing",
                f"segment {number}: segment file is missing",
            )
            continue

        if sidecar_path is None:
            report.add_failure(
                number,
                "missing",
                f"segment {number}: sidecar is missing",
            )
            continue

        doc, sidecar_error = sidecars.get(number, (None, None))
        if doc is None:
            report.add_failure(
                number,
                "sidecar",
                f"segment {number}: {sidecar_error or 'cannot read sidecar'}",
            )
            continue

        block_ok = True
        mismatched_fields = []

        doc_archive_id = doc.get("archive_id")
        if (
            expected_archive_id is not None
            and doc_archive_id is not None
            and str(doc_archive_id) != str(expected_archive_id)
        ):
            block_ok = False
            mismatched_fields.append("archive_id")
            report.add_failure(
                number,
                "archive_id",
                f"segment {number}: block belongs to a different archive",
                field="archive_id",
            )

        doc_key_id = str(doc.get("key_id"))
        key_entry = keys_mod.find_key(keyring, doc_key_id)
        signature_ok = False
        if key_entry is None:
            block_ok = False
            report.add_failure(
                number,
                "key",
                f"segment {number}: key material for {doc_key_id} is not loaded "
                "in the keyring",
                key_id=doc_key_id,
            )
        else:
            try:
                expected_signature = signature_for(key_entry, doc)
            except RotsealError as exc:
                block_ok = False
                report.add_failure(
                    number,
                    "key",
                    f"segment {number}: cannot use key {doc_key_id}: {exc}",
                    key_id=doc_key_id,
                )
            else:
                signature_ok = hmac.compare_digest(
                    str(doc.get("signature", "")), expected_signature
                )
                if not signature_ok:
                    block_ok = False

        if doc.get("alg") != "hmac-sha256":
            block_ok = False
            mismatched_fields.append("alg")
            report.add_failure(
                number,
                "algorithm",
                f"segment {number}: unsupported algorithm {doc.get('alg')!r}",
                field="alg",
            )
        if _int_or_none(doc.get("segment")) != number:
            block_ok = False
            mismatched_fields.append("segment")
            report.add_failure(
                number,
                "segment",
                f"segment {number}: sidecar names block {doc.get('segment')!r}",
                field="segment",
            )
        if doc.get("segment_file") != segment.name:
            block_ok = False
            mismatched_fields.append("segment_file")
            report.add_failure(
                number,
                "segment_file",
                f"segment {number}: sidecar names {doc.get('segment_file')!r}",
                field="segment_file",
            )

        try:
            raw = segment.read_bytes()
        except OSError as exc:
            report.add_failure(
                number, "records", f"segment {number}: unreadable records: {exc}"
            )
            continue

        sealed_length = _int_or_none(doc.get("sealed_length"))
        if sealed_length != len(raw):
            block_ok = False
            mismatched_fields.append("sealed_length")
            report.add_failure(
                number,
                "sealed_length",
                f"segment {number}: content beyond the sealed length "
                f"(sidecar {sealed_length}, file {len(raw)} bytes)",
                field="sealed_length",
            )

        try:
            records = read_records(segment)
        except RotsealError as exc:
            block_ok = False
            report.add_failure(
                number, "records", f"segment {number}: {exc}", field="records"
            )
            records = None

        measured_count = None
        measured_start = None
        measured_end = None
        if records is not None:
            measured_count = len(records)
            declared_count = _int_or_none(doc.get("count"))
            if measured_count != declared_count:
                block_ok = False
                mismatched_fields.append("count")
                report.add_failure(
                    number,
                    "count",
                    f"segment {number}: record count does not match the sidecar "
                    f"(sidecar {declared_count}, file {measured_count})",
                    field="count",
                )

            seqs = []
            seq_ok = True
            for position, record in enumerate(records, start=1):
                if "seq" not in record:
                    block_ok = False
                    seq_ok = False
                    report.add_failure(
                        number,
                        "seq",
                        f"segment {number}: record {position} has no seq field",
                        field="seq",
                    )
                    continue
                try:
                    seqs.append(int(record["seq"]))
                except (TypeError, ValueError):
                    block_ok = False
                    seq_ok = False
                    report.add_failure(
                        number,
                        "seq",
                        f"segment {number}: record {position} has a non-integer seq",
                        field="seq",
                    )

            if seq_ok and seqs:
                measured_start = seqs[0]
                measured_end = seqs[-1]
                declared_start = _int_or_none(doc.get("seq_start"))
                declared_end = _int_or_none(doc.get("seq_end"))
                if declared_start != measured_start:
                    block_ok = False
                    mismatched_fields.append("seq_start")
                    report.add_failure(
                        number,
                        "seq_start",
                        f"segment {number}: seq_start does not match the file "
                        f"(sidecar {declared_start}, file {measured_start})",
                        field="seq_start",
                    )
                if declared_end != measured_end:
                    block_ok = False
                    mismatched_fields.append("seq_end")
                    report.add_failure(
                        number,
                        "seq_end",
                        f"segment {number}: seq_end does not match the file "
                        f"(sidecar {declared_end}, file {measured_end})",
                        field="seq_end",
                    )
                for position in range(1, len(seqs)):
                    if seqs[position] != seqs[position - 1] + 1:
                        block_ok = False
                        report.add_failure(
                            number,
                            "sequence",
                            f"segment {number}: sequence gap between "
                            f"{seqs[position - 1]} and {seqs[position]}",
                            field="seq",
                        )
                        break
                measured_seq_end[number] = measured_end

            try:
                measured_digest = compute_digest(records)
            except (TypeError, ValueError) as exc:
                block_ok = False
                report.add_failure(
                    number,
                    "records_digest",
                    f"segment {number}: cannot digest records: {exc}",
                    field="records_digest",
                )
            else:
                if doc.get("records_digest") != measured_digest:
                    block_ok = False
                    mismatched_fields.append("records_digest")
                    report.add_failure(
                        number,
                        "records_digest",
                        f"segment {number}: records digest does not match the sidecar",
                        field="records_digest",
                    )

        if key_entry is not None and not signature_ok:
            block_ok = False
            report.add_failure(
                number,
                "signature",
                f"segment {number}: signature does not match the signed fields",
                fields=sorted(set(mismatched_fields)),
            )

        try:
            recomputed_tip = tip_for(number, doc)
        except (TypeError, ValueError, json.JSONDecodeError) as exc:
            block_ok = False
            report.add_failure(
                number,
                "tip",
                f"segment {number}: cannot recompute tip: {exc}",
                field="tip",
            )
        else:
            if doc.get("tip") != recomputed_tip:
                block_ok = False
                mismatched_fields.append("tip")
                report.add_failure(
                    number,
                    "tip",
                    f"segment {number}: tip does not match the block identity",
                    field="tip",
                )

        if number == 1:
            expected_prev = None
        elif (number - 1) in tips:
            expected_prev = tips[number - 1]
        elif (number - 1) in tombstones:
            expected_prev = tombstones[number - 1].get("tip")
            if expected_prev is None:
                expected_prev = _UNKNOWN
        else:
            expected_prev = _UNKNOWN

        if expected_prev is _UNKNOWN:
            block_ok = False
            report.add_failure(
                number,
                "prev_tip",
                f"segment {number}: cannot resolve the preceding block's tip",
                field="prev_tip",
            )
        elif doc.get("prev_tip") != expected_prev:
            block_ok = False
            report.add_failure(
                number,
                "prev_tip",
                f"segment {number}: prev_tip does not match the preceding block",
                field="prev_tip",
            )

        predecessor_end = None
        if (number - 1) in measured_seq_end:
            predecessor_end = measured_seq_end[number - 1]
        elif (number - 1) in declared_seq_end:
            predecessor_end = declared_seq_end[number - 1]
        elif (number - 1) in tombstones:
            predecessor_end = _int_or_none(tombstones[number - 1].get("seq_end"))
        if (
            measured_start is not None
            and predecessor_end is not None
            and measured_start != predecessor_end + 1
        ):
            block_ok = False
            report.add_failure(
                number,
                "sequence",
                f"segment {number}: seq_start {measured_start} does not follow "
                f"the preceding seq_end {predecessor_end}",
                field="seq_start",
            )

        if block_ok:
            report.add_checked(
                number,
                doc_key_id,
                measured_count,
                measured_start,
                measured_end,
            )

    present = sorted(number for number in range(1, highest + 1) if number in entries)
    if present:
        first = present[0]
        if first > 1:
            prefix = [number for number in range(1, first) if number in tombstones]
            if len(prefix) == first - 1:
                windows = sorted(
                    {
                        str(tombstones[number].get("window"))
                        for number in prefix
                        if tombstones[number].get("window")
                    }
                )
                suffix = f" (window {', '.join(windows)})" if windows else ""
                report.add_note(
                    f"first {first - 1} block(s) (segments 1..{first - 1}) "
                    f"were removed under the retention policy{suffix}"
                )

    if not report.checked and report.ok:
        report.add_note("nothing to verify in the requested range")
    return report
