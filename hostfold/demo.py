"""Print the canonicalization and policy decision table.

Run from the repository root::

    python -m hostfold.demo
"""

from __future__ import annotations

from pathlib import Path

from .errors import CanonicalizationError
from .parse import canonicalize
from .policy import load_policy

POLICY_PATH = Path(__file__).resolve().parent.parent / "policies" / "egress.yaml"

TARGETS = [
    "https://api.trusted.example/v1/items",
    "http://untrusted.test/",
    "http://127.0.0.1/",
    "http://127.1/",
    "http://2130706433/",
    "http://0x7f.0.0.1/",
    "http://0177.0.0.1/",
    "http://169.254.169.254/latest/meta-data/",
    "http://metadata.internal/",
    "http://metadata.internal./",
    "http://api.internal@evil.example/",
    "http://proxy-user@untrusted.test/",
    "http://nottrusted.example/",
    "http://example.com/",
    "http://Example.COM/",
    "http://bücher.example/",
    "http://evil。example/",
    "http://ｅｖｉｌ.example/",
    "http://" + "é" * 60 + ".test/",
    "http://[::1]:8080/",
    "http://example.com:0/",
    "http://example.com:8080/",
]

HEADERS = ("input", "scheme", "host", "port", "decision", "rule / reason")


def rows(policy) -> list[tuple[str, ...]]:
    """Return one display row per target."""
    table = []
    for target in TARGETS:
        try:
            canonical = canonicalize(target)
        except CanonicalizationError as exc:
            table.append((target, "-", "-", "-", "reject", exc.reason))
            continue
        action, rule_name = policy.decide(canonical.host, canonical.port)
        decision = "allow" if action == "allow" else "deny"
        table.append(
            (
                target,
                canonical.scheme,
                canonical.host,
                str(canonical.port),
                decision,
                rule_name or "default",
            )
        )
    return table


def render(table: list[tuple[str, ...]]) -> str:
    """Render *table* as a fixed-width text table."""
    widths = [len(header) for header in HEADERS]
    for row in table:
        for index, cell in enumerate(row):
            widths[index] = max(widths[index], len(cell))

    def line(cells: tuple[str, ...]) -> str:
        padded = [cell.ljust(widths[index]) for index, cell in enumerate(cells)]
        return "  ".join(padded).rstrip()

    parts = [line(HEADERS), line(tuple("-" * width for width in widths))]
    parts.extend(line(row) for row in table)
    return "\n".join(parts)


def main() -> int:
    policy = load_policy(POLICY_PATH)
    print(render(rows(policy)))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
