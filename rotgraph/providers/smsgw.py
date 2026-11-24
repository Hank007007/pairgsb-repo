"""SMS gateway adapter.

API shape::

    POST /sms/keys              {"slot": "prod"} -> {"key": {...}}
    POST /sms/keys/{id}/retire  {"keep_until": iso} -> {"ok": true}
    GET  /sms/keys/{id}                            -> {"status": "active"}
    GET  /sms/keys/{id}/ping  (X-Key)              -> 200 / 403

``keep_until`` is an ISO-8601 instant. An empty string means "stop it right
now"; the gateway treats a missing or empty value as immediate retirement.
"""

from __future__ import annotations

from typing import Any

from .base import Provider, ProviderError


class SmsGatewayProvider(Provider):
    def create(self, spec: dict[str, Any]) -> dict[str, Any]:
        answer = self._request(
            "POST",
            "/sms/keys",
            json={"slot": spec["slot"]},
            headers={"X-Api-Key": self.secret},
        )
        key = (answer or {}).get("key")
        if not isinstance(key, dict) or not key.get("id") or not key.get("secret"):
            raise ProviderError("smsgw: create returned no key material", body=answer)
        return {
            "id": key["id"],
            "secret": key["secret"],
            "slot": key.get("slot", spec["slot"]),
            "created_at": key.get("created_at"),
        }

    def revoke(self, spec: dict[str, Any], key_id: str, keep_until=None, **kwargs) -> dict[str, Any]:
        return self._request(
            "POST",
            f"/sms/keys/{key_id}/retire",
            json={"keep_until": ""},
            headers={"X-Api-Key": self.secret},
        )

    def describe(self, spec: dict[str, Any], key_id: str) -> dict[str, Any]:
        answer = self._request(
            "GET", f"/sms/keys/{key_id}", headers={"X-Api-Key": self.secret}
        )
        return {"state": (answer or {}).get("status", "unknown")}

    def probe(self, spec: dict[str, Any], key_id: str, secret: str, timeout: float = 10.0) -> bool:
        response = self.session.get(
            self._url(f"/sms/keys/{key_id}/ping"),
            headers={"X-Key": secret},
            timeout=timeout,
        )
        return getattr(response, "status_code", 0) == 200
