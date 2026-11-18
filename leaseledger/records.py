"""租约记录的导入与导出。

审计系统按 JSON 保存签发过的租约。导出时每一条记录是一行：

    {"id": "...", "subject": "...", "scopes": ["read"],
     "issued_at": "2026-01-01T12:00:00+00:00",
     "expires_at": "2026-01-01T12:10:00+00:00",
     "parent_lease_id": null, "revoked_at": null, "revoked_reason": null}

导入用来把审计侧的记录重新装进 :class:`~leaseledger.store.LeaseStore`，
以便对历史租约做状态核对。
"""

from __future__ import annotations

import json
from datetime import datetime
from pathlib import Path
from typing import Any, Iterable, Mapping

from .errors import InvalidLease
from .store import Lease


def _parse_time(value: Any, field: str) -> datetime | None:
    if value is None:
        return None
    if isinstance(value, datetime):
        return value
    if not isinstance(value, str):
        raise InvalidLease(f"{field} 必须是 ISO 8601 字符串: {value!r}")
    try:
        return datetime.fromisoformat(value)
    except ValueError as exc:
        raise InvalidLease(f"{field} 不是合法的 ISO 8601 时间: {value!r}") from exc


def _parse_optional_time(value: Any, field: str) -> datetime | None:
    return _parse_time(value, field)


def lease_from_record(record: Mapping[str, Any]) -> Lease:
    """把一条导出记录还原成 :class:`Lease`。"""

    if not isinstance(record, Mapping):
        raise InvalidLease("租约记录必须是对象")
    try:
        lease_id = record["id"]
        subject = record["subject"]
        scopes = record["scopes"]
        issued_at = record["issued_at"]
        expires_at = record["expires_at"]
    except KeyError as exc:
        raise InvalidLease(f"租约记录缺少字段 {exc.args[0]!r}") from None
    if not isinstance(lease_id, str) or not lease_id:
        raise InvalidLease(f"id 不合法: {lease_id!r}")
    if not isinstance(subject, str) or not subject:
        raise InvalidLease(f"subject 不合法: {subject!r}")
    if isinstance(scopes, str) or not isinstance(scopes, (list, tuple)):
        raise InvalidLease(f"scopes 必须是数组: {scopes!r}")
    parsed_scopes = tuple(scopes)
    if not parsed_scopes or not all(isinstance(s, str) for s in parsed_scopes):
        raise InvalidLease(f"scopes 不合法: {scopes!r}")
    issued = _parse_time(issued_at, "issued_at")
    expires = _parse_time(expires_at, "expires_at")
    if issued is None or expires is None:
        raise InvalidLease("issued_at / expires_at 不能为空")
    return Lease(
        id=lease_id,
        subject=subject,
        scopes=parsed_scopes,
        issued_at=issued,
        expires_at=expires,
        parent_lease_id=record.get("parent_lease_id"),
        revoked_at=_parse_optional_time(record.get("revoked_at"), "revoked_at"),
        revoked_reason=record.get("revoked_reason"),
    )


def lease_to_record(lease: Lease) -> dict[str, Any]:
    """把租约序列化成可写入审计日志的对象。"""

    return {
        "id": lease.id,
        "subject": lease.subject,
        "scopes": list(lease.scopes),
        "issued_at": lease.issued_at.isoformat(),
        "expires_at": lease.expires_at.isoformat(),
        "parent_lease_id": lease.parent_lease_id,
        "revoked_at": lease.revoked_at.isoformat() if lease.revoked_at else None,
        "revoked_reason": lease.revoked_reason,
    }


def load_records(path: str | Path) -> list[dict[str, Any]]:
    """从 JSON 文件读取记录数组。"""

    with open(path, "r", encoding="utf-8") as handle:
        data = json.load(handle)
    if not isinstance(data, list):
        raise InvalidLease("记录文件必须是数组")
    return data


def import_records(store: Any, records: Iterable[Mapping[str, Any]]) -> tuple[Lease, ...]:
    """把一批记录装进租约表，返回装入的租约。"""

    imported: list[Lease] = []
    for record in records:
        imported.append(store.put(lease_from_record(record)))
    return tuple(imported)
