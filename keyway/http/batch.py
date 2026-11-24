"""Batch decryption.

The batch endpoint shares the per-item decryption path with the single-item
endpoint but reports results as a list so callers can fan out.
"""

from __future__ import annotations

from ..errors import BatchTooLarge, InvalidRequest, KeywayError
from .routes import MAX_BATCH_ITEMS, Request, Response, _b64d, _context_from


def handle_batch(store, tenant_id: str, request: Request) -> Response:
    """Decrypt a list of items and return per-item results.

    §4.2 makes the item count a hard limit: an oversized or empty batch is
    rejected as a whole before any item is processed, with no partial
    results.
    """
    body = request.body
    items = body.get("items")
    if not isinstance(items, list):
        raise InvalidRequest("items must be a list")
    if len(items) == 0:
        raise InvalidRequest("items must not be empty")
    if len(items) > MAX_BATCH_ITEMS:
        raise BatchTooLarge(f"at most {MAX_BATCH_ITEMS} items per batch")

    results = []
    for item in items:
        results.append(_handle_item(store, tenant_id, item))
    return Response(200, {"results": results})


def _handle_item(store, tenant_id: str, item) -> dict:
    if not isinstance(item, dict):
        return {"ok": False, "error": {"code": "invalid_request"}}

    key_name = item.get("key_name")
    try:
        blob_text = item.get("blob")
        if not isinstance(blob_text, str) or not blob_text:
            raise InvalidRequest("blob required")
        context = _context_from(item)
        blob = _b64d(blob_text)
        plaintext = store.decrypt(tenant_id, blob, context)
    except KeywayError as exc:
        # §4.3: only the stable code and, when the request named one, the
        # key name are reported; no exception text, no plaintext lengths.
        error = {"code": exc.code}
        if isinstance(key_name, str) and key_name:
            error["key_name"] = key_name
        elif exc.key_name is not None:
            error["key_name"] = exc.key_name
        return {"ok": False, "error": error}
    return {"ok": True, "plaintext": plaintext.decode("utf-8", "replace")}
