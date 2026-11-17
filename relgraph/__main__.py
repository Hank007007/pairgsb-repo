"""Command line interface for relgraph."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from relgraph.resolver import Resolver
from relgraph.schema import load_schemas
from relgraph.store import TupleStore


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="relgraph",
        description="Evaluate relation-based authorization questions.",
    )
    parser.add_argument(
        "--store",
        type=Path,
        default=None,
        help="path to a JSON tuple store (kept in memory when omitted)",
    )
    parser.add_argument(
        "--schema",
        type=Path,
        default=None,
        help="directory containing schema documents",
    )

    commands = parser.add_subparsers(dest="command", required=True)

    add = commands.add_parser("add", help="add relation tuples")
    add.add_argument("tuples", nargs="+", help="tuple in object#relation@subject form")

    remove = commands.add_parser("remove", help="remove relation tuples")
    remove.add_argument("tuples", nargs="+", help="tuple in object#relation@subject form")

    check = commands.add_parser("check", help="evaluate one authorization question")
    check.add_argument("subject", help="subject reference, e.g. user:alice")
    check.add_argument("relation", help="relation name")
    check.add_argument("object", help="object reference, e.g. doc:1")

    commands.add_parser("list", help="list stored tuples")

    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)

    store = TupleStore(args.store)
    schemas = load_schemas(args.schema) if args.schema is not None else None
    resolver = Resolver(store, schemas)

    if args.command == "add":
        count = store.add_tuples(args.tuples)
        print(f"added {count}")
        return 0

    if args.command == "remove":
        count = store.remove_tuples(args.tuples)
        print(f"removed {count}")
        return 0

    if args.command == "list":
        for item in store:
            print(item)
        return 0

    decision = resolver.check(args.subject, args.relation, args.object)
    if decision.allowed:
        print("allow")
        return 0
    suffix = f" ({', '.join(decision.reasons)})" if decision.reasons else ""
    print(f"deny{suffix}")
    return 1


if __name__ == "__main__":
    sys.exit(main())
