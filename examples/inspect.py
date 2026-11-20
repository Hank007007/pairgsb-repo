"""Read certificate fixtures and print the identity service's view of them.

Run from the repository root::

    python examples/inspect.py
"""

from __future__ import annotations

import os
import sys

# The script is named after a standard-library module, so drop its own
# directory from the import path before anything imports the stdlib.
_HERE = os.path.dirname(os.path.abspath(__file__))
sys.path[:] = [
    entry
    for entry in sys.path
    if os.path.abspath(entry) not in (_HERE, os.path.dirname(_HERE))
]
sys.path.insert(0, os.path.dirname(_HERE))

from derbrace import (  # noqa: E402
    Certificate,
    ChainError,
    DERError,
    Budget,
    parse_certificate,
    subject_identity,
    verify_chain,
)

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
CERT_DIR = os.path.join(ROOT, "fixtures", "certs")
POOL_DIR = os.path.join(ROOT, "fixtures", "pool")
ANCHOR_DIR = os.path.join(ROOT, "fixtures", "anchors")


def load(path: str) -> bytes:
    with open(path, "rb") as handle:
        return handle.read()


def load_certs(directory: str) -> list:
    if not os.path.isdir(directory):
        return []
    certs = []
    for name in sorted(os.listdir(directory)):
        if not name.endswith(".der"):
            continue
        try:
            certs.append(parse_certificate(load(os.path.join(directory, name))))
        except (DERError, ValueError):
            continue
    return certs


def _printable(text: str) -> str:
    return "".join(
        ch
        if ch.isprintable() and ch.isascii()
        else "\\u%04x" % ord(ch)
        for ch in text
    )


def show(cert: Certificate) -> None:
    identity = subject_identity(cert.subject, cert.san)
    print("  subject: %s" % _printable(cert.subject))
    print("  issuer: %s" % _printable(cert.issuer))
    print("  serial: %d" % cert.serial)
    print("  version: %d" % cert.version)
    print("  notBefore: %s" % cert.not_before.isoformat())
    print("  notAfter: %s" % cert.not_after.isoformat())
    print("  san: %s" % (", ".join(_printable(s) for s in cert.san) if cert.san else "-"))
    print("  identity: %s" % (_printable(identity) if identity is not None else "-"))


def chain_verdict(cert: Certificate, intermediates: list, anchors: list) -> str:
    try:
        result = verify_chain(cert, intermediates, anchors)
    except ChainError as exc:
        return "rejected: %s" % exc
    except Exception as exc:  # noqa: BLE001 - report, do not crash
        return "error: %s: %s" % (type(exc).__name__, exc)
    names = " -> ".join(c.subject for c in result.chain)
    return "accepted: %s" % names


def main() -> int:
    if not os.path.isdir(CERT_DIR):
        print("missing fixture directory: %s" % CERT_DIR)
        return 1
    intermediates = load_certs(POOL_DIR)
    anchors = load_certs(ANCHOR_DIR)
    failed = 0
    for name in sorted(os.listdir(CERT_DIR)):
        if not name.endswith(".der"):
            continue
        print("== %s" % name)
        data = load(os.path.join(CERT_DIR, name))
        budget = Budget()
        try:
            cert = parse_certificate(data, budget=budget)
        except DERError as exc:
            print("  error: %s: %s" % (type(exc).__name__, exc))
            failed += 1
            continue
        except Exception as exc:  # noqa: BLE001 - report, do not crash
            print("  error: %s: %s" % (type(exc).__name__, exc))
            failed += 1
            continue
        show(cert)
        print("  chain: %s" % chain_verdict(cert, intermediates, anchors))
    print()
    print("%d certificate file(s) rejected" % failed)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
