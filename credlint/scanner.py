"""The scanning pipeline.

``Scanner`` ties together the file walker, the rule table, entropy scoring and
one-layer decoding.  A finding records where a credential was seen plus a
masked rendering of the value.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path

from .decode import maybe_decoded
from .entropy import is_random_looking, shannon
from .ignorefile import IgnoreFile
from .redact import mask
from .rules import RuleSet
from .walker import MAX_FILE_BYTES, iter_files, read_text


@dataclass
class Finding:
    """A single suspected credential."""

    path: str
    line: int
    rule_id: str
    preview: str
    entropy: float = 0.0
    context: str = ""
    source: str = "file"


@dataclass
class Scanner:
    """Scan text or directories with a rule table and an allowlist."""

    rules: RuleSet = field(default_factory=RuleSet.load)
    ignore: IgnoreFile = field(default_factory=lambda: IgnoreFile.load(None))
    threshold: float = 3.0

    def scan_line(
        self,
        line: str,
        path: str = "<memory>",
        lineno: int = 0,
        source: str = "file",
    ) -> list[Finding]:
        """Return findings for a single line."""

        results: list[Finding] = []
        for rule, pattern in self.rules:
            for match in pattern.finditer(line):
                try:
                    value = match.group(rule.group)
                except IndexError:  # pragma: no cover - malformed rule table
                    continue
                if not value:
                    continue

                score = shannon(line)
                if rule.entropy and not is_random_looking(line, self.threshold):
                    continue

                results.append(
                    Finding(
                        path=path,
                        line=lineno,
                        rule_id=rule.id,
                        preview=mask(value),
                        entropy=score,
                        context=line.rstrip("\n"),
                        source=source,
                    )
                )
        return results

    def score(self, text: str) -> float:
        """Return the entropy score of ``text``."""

        return shannon(text)

    def scan_text(
        self, text: str, path: str = "<memory>", source: str = "file"
    ) -> list[Finding]:
        """Scan every line of ``text``."""

        findings: list[Finding] = []
        for lineno, line in enumerate(text.splitlines(), start=1):
            findings.extend(
                self.scan_line(line, path=path, lineno=lineno, source=source)
            )
        return findings

    def scan_file(
        self, path: str | Path, root: str | Path | None = None
    ) -> list[Finding]:
        """Scan a single file, honouring the allowlist."""

        file_path = Path(path)
        if root is not None and Path(root).is_dir():
            try:
                relpath = (
                    file_path.resolve().relative_to(Path(root).resolve()).as_posix()
                )
            except ValueError:
                relpath = file_path.name
        else:
            relpath = file_path.name

        if self.ignore.matches(relpath):
            return []

        text = read_text(file_path)
        if len(text) > MAX_FILE_BYTES * 1024:
            return []

        findings = self.scan_text(text, path=relpath)
        findings.extend(self._scan_decoded(text, relpath))
        return findings

    def scan_path(self, root: str | Path) -> list[Finding]:
        """Scan every candidate file below ``root``."""

        findings: list[Finding] = []
        for path in iter_files(root):
            findings.extend(self.scan_file(path, root=root))
        return findings

    def _scan_decoded(self, text: str, path: str) -> list[Finding]:
        """Re-run the rules over a decoded blob when the whole text is encoded."""

        decoded = maybe_decoded(text)
        if decoded is text:
            return []
        return self.scan_text(decoded, path=path, source="decoded")


def scan_text(text: str, path: str = "<memory>") -> list[Finding]:
    """Convenience wrapper around :meth:`Scanner.scan_text`."""

    return Scanner().scan_text(text, path=path)


def scan_path(root: str | Path, ignore: str | Path | None = None) -> list[Finding]:
    """Convenience wrapper around :meth:`Scanner.scan_path`."""

    if ignore is None:
        candidate = Path(root) / ".credlintignore"
        if Path(root).is_dir() and candidate.exists():
            ignore = candidate
    scanner = Scanner(rules=RuleSet.load(), ignore=IgnoreFile.load(ignore))
    return scanner.scan_path(root)
