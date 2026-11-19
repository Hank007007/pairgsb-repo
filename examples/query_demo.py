"""End-to-end demo: compile and translate a representative filter.

Run with::

    python examples/query_demo.py
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from filtergate import FilterGate, MongoAdapter  # noqa: E402

TENANT = "acme-prod"

FILTER = {
    "$and": [
        {"level": {"$in": ["error", "fatal"]}},
        {
            "$or": [
                {"status_code": {"$gte": 500}},
                {"latency_ms": {"$gt": 2000}},
            ]
        },
        {"service": {"$eq": "checkout"}, "timestamp": {"$gte": 1760000000000}},
    ]
}


def main() -> int:
    adapter = MongoAdapter()
    result = FilterGate(adapter).compile(FILTER, tenant_id=TENANT)

    if not result.ok:
        print(f"compile failed: {result.error}", file=sys.stderr)
        return 1

    print(json.dumps(result.query, indent=2, sort_keys=True))
    print(f"\nadapter invocations: {len(adapter.calls)}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
