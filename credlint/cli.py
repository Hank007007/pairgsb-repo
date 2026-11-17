"""Command line entry point for credlint."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from .diff import parse_diff
from .ignorefile import IgnoreFile
from .report import render
from .rules import RuleSet
from .scanner import Finding, Scanner


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="credlint",
        description="Scan a directory or a unified diff for suspected credentials.",
    )
    parser.add_argument("target", nargs="?", default=".", help="directory, file or diff to scan")
    parser.add_argument("--diff", action="store_true", help="treat the target as a unified diff")
    parser.add_argument("--rules", default=None, help="path to a rule table (JSON)")
    parser.add_argument("--ignore", default=None, help="path to an allowlist file")
    parser.add_argument("--format", dest="fmt", choices=("text", "json"), default="text")
    parser.add_argument(
        "--fail-on-findings",
        action="store_true",
        help="exit with status 1 when anything is reported",
    )
    return parser


def scan_diff(path: Path, scanner: Scanner) -> list[Finding]:
    """Return findings for the added lines of the diff at ``path``."""

    text = path.read_text(encoding="utf-8", errors="replace")
    findings: list[Finding] = []
    for added in parse_diff(text):
        findings.extend(
            scanner.scan_line(added.text, path=added.path, lineno=added.line, source="diff")
        )
    return findings


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    target = Path(args.target)
    rules = RuleSet.load(args.rules)

    ignore_path = args.ignore
    if ignore_path is None and target.is_dir():
        candidate = target / ".credlintignore"
        if candidate.exists():
            ignore_path = candidate
    ignore = IgnoreFile.load(ignore_path)
    scanner = Scanner(rules=rules, ignore=ignore)

    if args.diff:
        findings = scan_diff(target, scanner)
    else:
        findings = scanner.scan_path(target)

    print(render(findings, fmt=args.fmt))
    if findings and args.fail_on_findings:
        return 1
    return 0


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
