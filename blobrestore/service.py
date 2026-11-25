"""The blobrestore sidecar service.

Producers hand a blob to :func:`revive`; the service validates it, restores the
object graph, and turns the graph into a cache key.  See ``docs/service.md``.
"""

from __future__ import annotations

import json
import logging
from http import HTTPStatus

from blobrestore.errors import BlobError
from blobrestore.ext import BytesValue
from blobrestore.model.reencode import graph_key
from blobrestore.model.typemap import load_type_table, resolve_type
from blobrestore.wire.format import read_header
from blobrestore.wire.restore import restore

logger = logging.getLogger("blobrestore")

MEMORY_LIMIT = 64 << 20  # 64 MiB, see docs/service.md §2.1


def _type_factory(table: dict[int, str]):
    """Return a ``type_id -> type name`` resolver backed by *table*."""

    def resolve(type_id: int) -> str:
        return resolve_type(table, type_id)

    return resolve


def revive(blob: bytes) -> dict:
    """Restore *blob* and return the HTTP-shaped result document."""
    try:
        if len(blob) > MEMORY_LIMIT:
            raise BlobError("blob exceeds the decode memory budget")
        header = read_header(blob)
        table = load_type_table(blob, header.type_table_off, header.ref_table_off)
        value = restore(blob, table, resolve_type=_type_factory(table))
        return {
            "ok": True,
            "value": _jsonable(value),
            "key": graph_key(value),
        }
    except Exception:
        logger.warning("revive failed for blob %r", repr(blob[:256]))
        return {"ok": False, "error": "blob could not be restored"}


def _jsonable(value):
    """Render a restored graph as plain JSON data."""
    if isinstance(value, BytesValue):
        return {"__bytes__": bytes(value).hex()}
    if isinstance(value, dict):
        return {str(k): _jsonable(v) for k, v in value.items()}
    if isinstance(value, (list, tuple)):
        return [_jsonable(v) for v in value]
    return value


def handle(request: dict) -> tuple[int, dict]:
    """Serve one request from the HTTP shell."""
    body = request.get("body") or b""
    if isinstance(body, str):
        body = body.encode("utf-8")
    result = revive(bytes(body))
    status = HTTPStatus.OK if result["ok"] else HTTPStatus.BAD_REQUEST
    return int(status), result


def dumps_response(result: dict) -> str:
    return json.dumps(result, sort_keys=True)
