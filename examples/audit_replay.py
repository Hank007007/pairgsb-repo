"""把审计侧导出的租约重新装进租约表并核对状态。

用法::

    python examples/audit_replay.py examples/exported_leases.json
"""

from __future__ import annotations

import sys
from datetime import datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from leaseledger import FixedClock, LeaseStore  # noqa: E402
from leaseledger.records import import_records, load_records  # noqa: E402


def main(argv: list[str]) -> int:
    path = argv[1] if len(argv) > 1 else str(
        Path(__file__).resolve().parent / "exported_leases.json"
    )
    clock = FixedClock(datetime(2026, 1, 1, 12, 10, tzinfo=timezone.utc))
    store = LeaseStore(clock)
    leases = import_records(store, load_records(path))
    print(f"imported {len(leases)} leases at {clock.now().isoformat()}")
    for lease in leases:
        state = "active" if store.is_active(lease) else "inactive"
        parent = lease.parent_lease_id or "-"
        print(f"  {lease.id:<18} scopes={','.join(lease.scopes):<20} parent={parent:<18} {state}")
    print("purged on sweep:", len(store.purge_expired()))
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
