"""Command-line walkthrough of the elevation workflow."""

from __future__ import annotations

import argparse
from datetime import timedelta

from .errors import GlassbreakError
from .policy import load_policy
from .service import GlassbreakService


def _demo() -> int:
    service = GlassbreakService(load_policy())

    request = service.create_request(
        subject="alice",
        requested_scopes=["db:write", "logs:read"],
        reason="replica lag during incident INC-4471",
        duration=timedelta(hours=1),
    )
    print(f"request {request.request_id} from {request.subject}")
    print(f"  wants: {sorted(request.requested_scopes)}")
    print(f"  for:   {request.duration}")

    for voter in ("bob", "carol"):
        try:
            service.approve(request.request_id, voter)
            print(f"  vote from {voter}: recorded")
        except GlassbreakError as error:
            print(f"  vote from {voter}: refused ({error})")

    try:
        issued = service.issue_credential(request.request_id)
    except GlassbreakError as error:
        print(f"credential: refused ({error})")
        return 1

    print(f"credential {issued.credential_id}")
    print(f"  scopes:  {sorted(issued.scopes)}")
    print(f"  expires: {issued.expires_at.isoformat()}")

    print("audit:")
    for entry in service.audit:
        print(f"  {entry.at.isoformat()} {entry.event:<18} {entry.actor}")
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="glassbreak")
    parser.add_argument(
        "command",
        nargs="?",
        default="demo",
        choices=["demo"],
        help="workflow to run (default: demo)",
    )
    parser.parse_args(argv)
    return _demo()


if __name__ == "__main__":
    raise SystemExit(main())
