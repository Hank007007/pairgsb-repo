"""Signing service adapter.

API shape::

    POST /v1/keys          {"slot": "prod"}     -> {"key": {...}}
    POST /v1/keys/{id}/revoke {"keep_until": iso} -> {"ok": true}
    GET  /v1/keys/{id}                          -> {"state": "active"}
    GET  /v1/keys/{id}/check  (Bearer)          -> 200 / 401
"""

from __future__ import annotations

from typing import Any

from .base import Provider, ProviderError


class SignerProvider(Provider):
    def create(self, spec: dict[str, Any]) -> dict[str, Any]:
        answer = self._request(
            "POST",
            "/v1/keys",
            json={"slot": spec["slot"]},
            headers={"X-Api-Key": self.secret},
        )
        key = (answer or {}).get("key")
        if not isinstance(key, dict) or not key.get("id") or not key.get("secret"):
            raise ProviderError("signer: create returned no key material", body=answer)
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
            f"/v1/keys/{key_id}/revoke",
            json=payload,
            headers={"X-Api-Key": self.secret},
        )

    def describe(self, spec: dict[str, Any], key_id: str) -> dict[str, Any]:
        answer = self._request(
            "GET", f"/v1/keys/{key_id}", headers={"X-Api-Key": self.secret}
        )
        return {"state": (answer or {}).get("state", "unknown")}

    def probe(self, spec: dict[str, Any], key_id: str, secret: str, timeout: float = 10.0) -> bool:
        response = self.session.get(
            self._url(f"/v1/keys/{key_id}/check"),
            headers={"Authorization": f"Bearer {secret}"},
            timeout=timeout,
        )
        return getattr(response, "status_code", 0) == 200
