"""Tuple storage with optional JSON persistence."""

from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable, Iterator


@dataclass(frozen=True, order=True)
class Tuple:
    """One relationship tuple: ``object#relation@subject``."""

    object: str
    relation: str
    subject: str

    def __str__(self) -> str:
        return f"{self.object}#{self.relation}@{self.subject}"


def parse_tuple(value: str) -> Tuple:
    """Parse the textual tuple form used by the service APIs."""

    text = value.strip()
    if not text:
        raise ValueError("empty tuple")

    head, separator, subject = text.partition("@")
    if not separator or not subject:
        raise ValueError(f"invalid tuple: {value!r}")

    obj, separator, relation = head.partition("#")
    if not separator or not obj or not relation:
        raise ValueError(f"invalid tuple: {value!r}")
    if "#" in relation:
        raise ValueError(f"invalid tuple: {value!r}")

    _object_type(obj)
    return Tuple(obj, relation, subject)


def _object_type(reference: str) -> str:
    kind, separator, identifier = reference.partition(":")
    if not separator or not kind or not identifier:
        raise ValueError(f"invalid object reference: {reference!r}")
    return kind


class TupleStore:
    """An in-memory tuple set that can be backed by a JSON file."""

    def __init__(self, path: str | Path | None = None) -> None:
        self.path = Path(path) if path is not None else None
        self._tuples: set[Tuple] = set()
        self._revision = 0
        if self.path is not None and self.path.exists():
            self._load()

    @property
    def revision(self) -> int:
        """Monotonic counter describing the state of the tuple set."""

        return self._revision

    def add_tuples(self, entries: Iterable[Tuple | str]) -> int:
        """Add tuples and return how many were new."""

        added = 0
        for entry in entries:
            item = self._coerce(entry)
            if item not in self._tuples:
                self._tuples.add(item)
                added += 1
        if added:
            self._advance()
        self._persist()
        return added

    def remove_tuples(self, entries: Iterable[Tuple | str]) -> int:
        """Remove tuples and return how many were present."""

        removed = 0
        for entry in entries:
            item = self._coerce(entry)
            if item in self._tuples:
                self._tuples.discard(item)
                removed += 1
        if removed:
            self._advance()
        self._persist()
        return removed

    def subjects_for(self, obj: str, relation: str) -> set[str]:
        """Return the subject side of the matching tuples."""

        return {
            item.subject
            for item in self._tuples
            if item.object == obj and item.relation == relation
        }

    def all_tuples(self) -> tuple[Tuple, ...]:
        """Return every stored tuple in a deterministic order."""

        return tuple(sorted(self._tuples))

    def __len__(self) -> int:
        return len(self._tuples)

    def __iter__(self) -> Iterator[Tuple]:
        return iter(sorted(self._tuples))

    def __contains__(self, entry: object) -> bool:
        if isinstance(entry, Tuple):
            return entry in self._tuples
        if isinstance(entry, str):
            return parse_tuple(entry) in self._tuples
        return False

    def _advance(self) -> None:
        self._revision += 1

    @staticmethod
    def _coerce(entry: Tuple | str) -> Tuple:
        if isinstance(entry, Tuple):
            return entry
        return parse_tuple(entry)

    def _persist(self) -> None:
        if self.path is None:
            return
        self.path.parent.mkdir(parents=True, exist_ok=True)
        payload = {
            "version": 1,
            "tuples": [str(item) for item in self.all_tuples()],
        }
        self.path.write_text(
            json.dumps(payload, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )

    def _load(self) -> None:
        raw = self.path.read_text(encoding="utf-8")
        if not raw.strip():
            return
        payload = json.loads(raw)
        self._tuples = {parse_tuple(str(item)) for item in payload.get("tuples", [])}
