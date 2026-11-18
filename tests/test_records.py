"""租约记录的导入导出（正常路径）。"""

from __future__ import annotations

import json

import pytest

from leaseledger import InvalidLease, issue
from leaseledger.records import (
    import_records,
    lease_from_record,
    lease_to_record,
    load_records,
)

RECORD = {
    "id": "lease-1",
    "subject": "svc-orders",
    "scopes": ["read", "write"],
    "issued_at": "2026-01-01T12:00:00",
    "expires_at": "2026-01-01T13:00:00",
    "parent_lease_id": None,
    "revoked_at": None,
    "revoked_reason": None,
}


def test_roundtrip_record():
    lease = lease_from_record(RECORD)

    assert lease.id == "lease-1"
    assert lease.scopes == ("read", "write")
    assert lease.parent_lease_id is None


def test_export_then_import(store, policy, clock):
    lease = issue(
        store, policy, clock, subject="svc-orders", scopes="read", ttl_seconds=600
    )

    record = lease_to_record(lease)
    restored = lease_from_record(record)

    assert restored.id == lease.id
    assert restored.expires_at == lease.expires_at
    assert restored.scopes == lease.scopes


def test_import_records_into_store(store):
    imported = import_records(store, [RECORD])

    assert len(imported) == 1
    assert "lease-1" in store


def test_import_records_preserves_revocation(store):
    record = dict(RECORD, revoked_at="2026-01-01T12:30:00", revoked_reason="lost device")

    imported = import_records(store, [record])

    assert imported[0].revoked is True
    assert imported[0].revoked_reason == "lost device"


def test_import_records_preserves_parent(store):
    child = dict(RECORD, id="lease-2", parent_lease_id="lease-1")

    imported = import_records(store, [RECORD, child])

    assert imported[1].parent_lease_id == "lease-1"
    assert store.children_of("lease-1") == (imported[1],)


def test_missing_field_rejected():
    record = {k: v for k, v in RECORD.items() if k != "expires_at"}

    with pytest.raises(InvalidLease):
        lease_from_record(record)


def test_bad_timestamp_rejected():
    record = dict(RECORD, issued_at="not-a-time")

    with pytest.raises(InvalidLease):
        lease_from_record(record)


def test_scopes_must_be_array():
    record = dict(RECORD, scopes="read")

    with pytest.raises(InvalidLease):
        lease_from_record(record)


def test_load_records_file(tmp_path):
    path = tmp_path / "records.json"
    path.write_text(json.dumps([RECORD]), encoding="utf-8")

    records = load_records(path)

    assert records == [RECORD]
