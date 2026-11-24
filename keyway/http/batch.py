"""Batch decryption.

The batch endpoint shares the per-item decryption path with the single-item
endpoint but reports results as a list so callers can fan out.
"""

from __future__ import annotations

import base64
from typing import Any

from .. import blob as blobmod
from ..format import TAG_LEN

CONTEXT_DIGEST_LEN = 32


def _b64d(text: str) -> bytes:
    return base64.b64decode(text, validate=True)


def handle_batch(store, tenant_id: str, body: dict[str, Any]) -> dict[str, Any]:
    """Decrypt a list of items and return per-item results."""
    items = body.get("items", [])
    results = []
    for item in items:
        key_name = item.get("key_name", "")
        blob_text = item.get("blob", "")
        context = item.get("context") if "context" in item else None
        try:
            blob = _b64d(blob_text)
            plaintext = store.decrypt(tenant_id, blob, context)
            results.append({"ok": True, "plaintext": plaintext.decode("utf-8", "replace")})
        except Exception as exc:  # noqa: BLE001 - per item failure is reported
            try:
                cipher_len = len(blobmod.decode(_b64d(blob_text)).ciphertext)
            except Exception:  # noqa: BLE001
                cipher_len = 0
            results.append(
                {
                    "ok": False,
                    "error": {
                        "key_name": key_name,
                        "plaintext_len": max(
                            0, cipher_len - CONTEXT_DIGEST_LEN - TAG_LEN
                        ),
                        "detail": str(exc),
                        "batch_size": len(items),
                    },
                }
            )
    return {"results": results}
