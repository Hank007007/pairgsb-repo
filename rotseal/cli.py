"""Command line interface for rotseal."""

from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path

from . import __version__
from .chain import verify_chain
from .keys import active_key, add_key, keyring_path, load_keyring
from .retention import prune
from .rotate import rotate
from .sealing import seal
from .util import (
    RotsealError,
    active_path,
    ensure_archive,
    load_index,
    set_segments_readonly,
)

ARCHIVE_ENV = "ROTSEAL_ARCHIVE"
DEFAULT_ARCHIVE = "archive"


def _archive(args) -> Path:
    return Path(args.archive or os.environ.get(ARCHIVE_ENV) or DEFAULT_ARCHIVE)


def _keyring(args) -> dict:
    return load_keyring(keyring_path(args.keyring))


def cmd_init(args) -> int:
    archive = ensure_archive(_archive(args))
    print(f"archive ready: {archive}")
    return 0


def cmd_append(args) -> int:
    archive = ensure_archive(_archive(args))
    target = active_path(archive)
    lines = []
    if args.file:
        lines = Path(args.file).read_text(encoding="utf-8").splitlines()
    else:
        lines = [line for line in sys.stdin.read().splitlines()]
    with target.open("a", encoding="utf-8") as handle:
        for line in lines:
            if line.strip():
                handle.write(line.rstrip("\n") + "\n")
    return 0


def cmd_rotate(args) -> int:
    archive = _archive(args)
    created = rotate(
        archive,
        max_bytes=args.max_bytes,
        every=args.every,
        reason=args.reason,
    )
    for entry in created:
        print(f"rotated: segment {entry['number']} ({entry['bytes']} bytes)")
    if not created:
        print("nothing to rotate")
    return 0


def cmd_seal(args) -> int:
    archive = _archive(args)
    keyring = _keyring(args)
    sealed = seal(archive, keyring, key_id=args.key)
    set_segments_readonly(archive, True)
    for number in sealed:
        print(f"sealed: segment {number}")
    if not sealed:
        print("nothing to seal")
    else:
        print(f"sealed {len(sealed)} segment(s); archive is now read-only")
    return 0


def cmd_verify(args) -> int:
    archive = _archive(args)
    keyring = _keyring(args)
    report = verify_chain(
        archive,
        keyring,
        from_segment=args.from_segment,
        to_segment=args.to_segment,
    )
    if args.json:
        print(json.dumps(report.as_dict(), ensure_ascii=False, indent=2))
        return report.exit_code
    for item in report.checked:
        print(
            "ok: segment {segment} key={key_id} records={count} "
            "seq={seq_start}..{seq_end}".format(**item)
        )
    for item in report.warnings:
        print(f"warning: {item['message']}")
    for item in report.notes:
        print(f"note: {item['message']}")
    for item in report.failures:
        print(f"FAIL: {item['message']}")
    if report.ok:
        print(f"chain ok: {len(report.checked)} segment(s) verified")
    else:
        print(f"chain check failed: {len(report.failures)} problem(s)")
    return report.exit_code


def cmd_prune(args) -> int:
    archive = _archive(args)
    result = prune(archive, args.keep, dry_run=args.dry_run)
    verb = "would prune" if result["dry_run"] else "pruned"
    for number in result["removed"]:
        print(f"{verb}: segment {number}")
    for item in result.get("reported", []):
        print(f"warning: segment {item['segment']}: {item['reason']}")
    if not result["removed"]:
        print(f"nothing to prune (keep {result['window']})")
    return 0


def cmd_key_add(args) -> int:
    entry = add_key(keyring_path(args.keyring), key_id=args.key_id)
    print(f"added key: {entry['id']} (now active)")
    return 0


def cmd_key_list(args) -> int:
    keyring = load_keyring(keyring_path(args.keyring))
    for item in keyring.get("keys", []):
        marker = "*" if item.get("id") == keyring.get("active") else " "
        print(f"{marker} {item.get('id')} created={item.get('created')}")
    if not keyring.get("keys"):
        print("keyring is empty")
    return 0


def cmd_status(args) -> int:
    archive = _archive(args)
    if not archive.exists():
        print(f"no archive at {archive}")
        return 1
    index = load_index(archive)
    keyring = _keyring(args)
    try:
        current = active_key(keyring)
        active_id = current.get("id")
    except RotsealError:
        active_id = None
    print(f"archive: {archive}")
    print(f"active key: {active_id or '(none)'}")
    print(f"segments: {len(index.get('segments', []))}")
    for entry in index["segments"]:
        print(
            "  segment {number}: {bytes} bytes sealed={sealed} key={key}".format(
                number=entry.get("number"),
                bytes=entry.get("bytes", "?"),
                sealed=bool(entry.get("sealed")),
                key=entry.get("key_id", "-"),
            )
        )
    if active_path(archive).exists():
        print(f"active log: {active_path(archive).stat().st_size} bytes")
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="rotseal",
        description="Rotate, seal and verify local NDJSON application logs.",
    )
    parser.add_argument("--version", action="version", version=f"rotseal {__version__}")
    parser.add_argument(
        "--archive",
        default=None,
        help=f"archive directory (default: ${ARCHIVE_ENV} or ./{DEFAULT_ARCHIVE})",
    )
    parser.add_argument(
        "--keyring",
        default=None,
        help="path to the keyring file (default: $ROTSEAL_KEYRING)",
    )
    common = argparse.ArgumentParser(add_help=False)
    common.add_argument("--archive", default=argparse.SUPPRESS, help=argparse.SUPPRESS)
    key_common = argparse.ArgumentParser(add_help=False)
    key_common.add_argument(
        "--keyring", default=argparse.SUPPRESS, help=argparse.SUPPRESS
    )

    subparsers = parser.add_subparsers(dest="command", required=True)

    init = subparsers.add_parser("init", parents=[common], help="create an archive")
    init.set_defaults(func=cmd_init)

    append = subparsers.add_parser(
        "append", parents=[common], help="append NDJSON records to the active log"
    )
    append.add_argument("file", nargs="?", help="read records from this file")
    append.set_defaults(func=cmd_append)

    rot = subparsers.add_parser(
        "rotate", parents=[common], help="close the active log into segments"
    )
    rot.add_argument("--max-bytes", type=int, default=None, help="rotate by size")
    rot.add_argument("--every", default=None, help="rotate by time window, e.g. 1h")
    rot.add_argument("--reason", default="manual", help="reason recorded in the index")
    rot.set_defaults(func=cmd_rotate)

    sea = subparsers.add_parser(
        "seal", parents=[common, key_common], help="sign and freeze sealed segments"
    )
    sea.add_argument("--key", default=None, help="key id to seal with")
    sea.set_defaults(func=cmd_seal)

    ver = subparsers.add_parser(
        "verify", parents=[common, key_common], help="verify the sealed chain"
    )
    ver.add_argument("--from", dest="from_segment", type=int, default=0)
    ver.add_argument("--to", dest="to_segment", type=int, default=None)
    ver.add_argument("--json", action="store_true", help="emit a JSON report")
    ver.set_defaults(func=cmd_verify)

    pru = subparsers.add_parser(
        "prune", parents=[common], help="delete segments beyond the retention window"
    )
    pru.add_argument("--keep", required=True, help="retention window, e.g. 30d")
    pru.add_argument("--dry-run", action="store_true")
    pru.set_defaults(func=cmd_prune)

    sta = subparsers.add_parser(
        "status", parents=[common, key_common], help="describe the archive"
    )
    sta.set_defaults(func=cmd_status)

    key = subparsers.add_parser("key", help="manage the keyring")
    key_sub = key.add_subparsers(dest="key_command", required=True)
    key_add = key_sub.add_parser("add", parents=[key_common], help="add a new key")
    key_add.add_argument("--id", dest="key_id", default=None)
    key_add.set_defaults(func=cmd_key_add)
    key_list = key_sub.add_parser("list", parents=[key_common], help="list keys")
    key_list.set_defaults(func=cmd_key_list)

    return parser


def main(argv=None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    try:
        return args.func(args)
    except RotsealError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
