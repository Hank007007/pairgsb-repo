"""Command line interface for subsetwitness.

Commands::

    subsetctl seal    --chain audit.log --input events.json --since ... --until ... --period-id ... --key ...
    subsetctl extract --chain audit.log --actor alice --since ... --until ... --period-id ... --out pkg.json
    subsetctl verify  pkg.json --chain audit.log --key ...
    subsetctl chain   --chain audit.log
"""

from __future__ import annotations

import argparse
import json
import sys

from . import PACKAGE_VERSION
from .chain import append_commit, find_commitment, read_chain, seal, verify_chain
from .extract import extract, make_commitment


def _load_json(path: str):
    with open(path, "r", encoding="utf-8") as handle:
        return json.load(handle)


def _write_json(path: str, payload) -> None:
    with open(path, "w", encoding="utf-8", newline="\n") as handle:
        json.dump(payload, handle, ensure_ascii=False, indent=1, sort_keys=True)
        handle.write("\n")


def cmd_seal(args: argparse.Namespace) -> int:
    events = _load_json(args.input)
    events = [record for record in events if record.get("kind", "event") == "event"]

    try:
        existing = read_chain(args.chain)
    except FileNotFoundError:
        existing = []

    start_seq = max((record.get("seq", 0) for record in existing), default=0)
    numbered = []
    for offset, record in enumerate(events, start=1):
        entry = dict(record)
        if "seq" not in entry:
            entry["seq"] = start_seq + offset
        entry.setdefault("kind", "event")
        numbered.append(entry)

    from .chain import GENESIS_HASH, write_chain

    prev = existing[-1]["hash"] if existing else GENESIS_HASH
    combined = existing + seal(numbered, start_prev=prev)
    write_chain(args.chain, combined)
    sealed_new = combined[len(existing):]
    commitment = make_commitment(sealed_new, args.period_id, args.since, args.until, args.key)
    entry = append_commit(args.chain, commitment)
    print(json.dumps({"period_id": entry["period_id"], "hash": entry["hash"]}, sort_keys=True))
    return 0


def cmd_extract(args: argparse.Namespace) -> int:
    records = read_chain(args.chain)
    commitment = find_commitment(records, args.period_id)
    package = extract(records, args.actor, args.since, args.until, commitment)
    package["version"] = PACKAGE_VERSION
    _write_json(args.out, package)
    print(
        json.dumps(
            {
                "out": args.out,
                "actor": args.actor,
                "count": package["count"],
                "bucket_root": package["bucket_root"],
            },
            sort_keys=True,
        )
    )
    return 0


def cmd_verify(args: argparse.Namespace) -> int:
    from .verify import overall_ok, verify_package

    package = _load_json(args.package)
    records = read_chain(args.chain) if args.chain else None
    checks = verify_package(package, records, key=args.key)

    for check in checks:
        suffix = ""
        if check.get("seqs"):
            suffix = " seqs=" + ",".join(str(seq) for seq in check["seqs"])
        print(f"{check['status']:4}  {check['name']:16}  {check['detail']}{suffix}")

    return 0 if overall_ok(checks) else 1


def cmd_chain(args: argparse.Namespace) -> int:
    records = read_chain(args.chain)
    ok, bad = verify_chain(records)
    print(f"records={len(records)} chain={'ok' if ok else 'broken at ' + str(bad)}")
    return 0 if ok else 1


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="subsetctl")
    sub = parser.add_subparsers(dest="command", required=True)

    seal_p = sub.add_parser("seal", help="seal a period into the chain")
    seal_p.add_argument("--chain", required=True)
    seal_p.add_argument("--input", required=True)
    seal_p.add_argument("--since", required=True)
    seal_p.add_argument("--until", required=True)
    seal_p.add_argument("--period-id", required=True)
    seal_p.add_argument("--key", required=True)
    seal_p.set_defaults(func=cmd_seal)

    extract_p = sub.add_parser("extract", help="extract a disclosure package")
    extract_p.add_argument("--chain", required=True)
    extract_p.add_argument("--actor", required=True)
    extract_p.add_argument("--since", required=True)
    extract_p.add_argument("--until", required=True)
    extract_p.add_argument("--period-id", required=True)
    extract_p.add_argument("--out", required=True)
    extract_p.set_defaults(func=cmd_extract)

    verify_p = sub.add_parser("verify", help="verify a disclosure package")
    verify_p.add_argument("package")
    verify_p.add_argument("--chain")
    verify_p.add_argument("--key")
    verify_p.set_defaults(func=cmd_verify)

    chain_p = sub.add_parser("chain", help="inspect a chain file")
    chain_p.add_argument("--chain", required=True)
    chain_p.set_defaults(func=cmd_chain)

    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    return args.func(args)


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
