"""Pattern table used by the scanner.

Rules live in a JSON document so that security teams can extend the table
without touching code.  Every rule has an id, a regular expression and the
capture group that holds the credential.  Rules may additionally require the
captured value to look randomly generated (``"entropy": true``).
"""

from __future__ import annotations

import json
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable, Iterator

RULES_DIR = Path(__file__).resolve().parent.parent / "rules"
DEFAULT_RULES_PATH = RULES_DIR / "default.json"


@dataclass(frozen=True)
class Rule:
    """A single detection rule."""

    id: str
    pattern: str
    group: int = 0
    entropy: bool = False
    description: str = ""


class RuleSet:
    """A compiled collection of :class:`Rule` objects."""

    def __init__(self, rules: Iterable[Rule]) -> None:
        self.rules: list[tuple[Rule, re.Pattern[str]]] = [
            (rule, re.compile(rule.pattern)) for rule in rules
        ]

    def __iter__(self) -> Iterator[tuple[Rule, re.Pattern[str]]]:
        return iter(self.rules)

    def __len__(self) -> int:
        return len(self.rules)

    @classmethod
    def load(cls, path: str | Path | None = None) -> "RuleSet":
        source = Path(path) if path is not None else DEFAULT_RULES_PATH
        with source.open("r", encoding="utf-8") as handle:
            document = json.load(handle)
        entries = document.get("rules", document) if isinstance(document, dict) else document
        rules = [
            Rule(
                id=entry["id"],
                pattern=entry["pattern"],
                group=int(entry.get("group", 0)),
                entropy=bool(entry.get("entropy", False)),
                description=str(entry.get("description", "")),
            )
            for entry in entries
        ]
        return cls(rules)


def load_rules(path: str | Path | None = None) -> list[Rule]:
    """Return the rule list from ``path`` (or the bundled table)."""

    return [rule for rule, _ in RuleSet.load(path)]
