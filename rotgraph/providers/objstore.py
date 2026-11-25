"""Object storage adapter.

API shape::

    POST /api/credentials        {"slot": "prod"} -> {"credentials": [ ... ]}
    POST /api/credentials/{id}/revoke {"keep_until": iso} -> {"ok": true}
    GET  /api/credentials/{id}                     -> {"state": "active"}
    HEAD /api/credentials/{id}  (Bearer)           -> 200 / 403

``/api/credentials`` answers with every credential the account has ever held
for the slot, newest last, so a caller has to identify the one it just minted.
"""

from __future__ import annotations

from datetime import datetime
from typing import Any

from .base import Provider, ProviderError


class ObjStoreProvider(Provider):
    def create(self, spec: dict[str, Any]) -> dict[str, Any]:
        answer = self._request(
            "POST",
            "/api/credentials",
            json={"slot": spec["slot"]},
            headers={"Authorization": f"Bearer {self.secret}"},
        )
        credentials = (answer or {}).get("credentials") or []
        if not isinstance(credentials, list) or not credentials:
            raise ProviderError("objstore: create returned no credentials", body=answer)
        key = _select_minted_credential(credentials, spec)
        return {
            "id": key["id"],
            "secret": key["secret"],
            "slot": key.get("slot", spec["slot"]),
            "created_at": key.get("created_at"),
        }

    def revoke(self, spec: dict[str, Any], key_id: str, keep_until=None, **kwargs) -> dict[str, Any]:
        payload: dict[str, Any] = {}
        if keep_until:
            payload["keep_until"] = keep_until
        return self._request(
            "POST",
            f"/api/credentials/{key_id}/revoke",
            json=payload,
            headers={"Authorization": f"Bearer {self.secret}"},
        )

    def describe(self, spec: dict[str, Any], key_id: str) -> dict[str, Any]:
        answer = self._request(
            "GET",
            f"/api/credentials/{key_id}",
            headers={"Authorization": f"Bearer {self.secret}"},
        )
        return {"state": (answer or {}).get("state", "unknown")}

    def probe(self, spec: dict[str, Any], key_id: str, secret: str, timeout: float = 10.0) -> bool:
        response = self.session.head(
            self._url(f"/api/credentials/{key_id}"),
            headers={"Authorization": f"Bearer {secret}"},
            timeout=timeout,
        )
        return getattr(response, "status_code", 0) == 200


def _parse_timestamp(value: Any) -> datetime | None:
    if not isinstance(value, str) or not value:
        return None
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None
    if parsed.tzinfo is None:
        return parsed.replace(tzinfo=datetime.now().astimezone().tzinfo)
    return parsed


def _select_minted_credential(
    credentials: list[Any], spec: dict[str, Any]
) -> dict[str, Any]:
    """Pick this request's credential without relying on a list position."""

    keys = [item for item in credentials if isinstance(item, dict) and item.get("id")]
    if not keys:
        raise ProviderError("objstore: create returned no usable credentials")

    slot = spec.get("slot")
    same_slot = [item for item in keys if not slot or item.get("slot", slot) == slot]
    candidates = same_slot or keys

    marker_names = ("just_created", "is_new", "newly_created", "minted", "new")
    for marker in marker_names:
        marked = [item for item in candidates if item.get(marker) is True]
        if len(marked) == 1:
            return marked[0]

    dated: list[tuple[datetime, int, dict[str, Any]]] = []
    for index, item in enumerate(candidates):
        parsed = _parse_timestamp(item.get("created_at"))
        if parsed is not None:
            dated.append((parsed, index, item))
    if dated:
        dated.sort(key=lambda entry: (entry[0], entry[1]))
        return dated[-1][2]

    id_marker = [
        item
        for item in candidates
        if str(item.get("id", "")).endswith("_new") or "_new_" in str(item.get("id", ""))
    ]
    if len(id_marker) == 1:
        return id_marker[0]

    raise ProviderError(
        "objstore: could not identify the newly minted credential",
        body={"credentials": credentials},
    )
