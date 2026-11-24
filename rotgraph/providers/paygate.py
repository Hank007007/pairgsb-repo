"""Payment gateway adapter.

API shape::

    POST /gateway/credentials      {"slot": "prod"} -> {"id": ..., "secret": ...}
    DELETE /gateway/credentials/{id}?effective_at=iso -> {"ok": true}
    GET  /gateway/credentials/{id}                    -> {"status": "active"}
    GET  /gateway/credentials/{id}/verify (Bearer)     -> 200 / 401
"""

from __future__ import annotations

from typing import Any

from .base import Provider, ProviderError


class PaygateProvider(Provider):
    def create(self, spec: dict[str, Any]) -> dict[str, Any]:
        answer = self._request(
            "POST",
            "/gateway/credentials",
            json={"slot": spec["slot"]},
            headers={"Authorization": f"Bearer {self.secret}"},
        )
        if not isinstance(answer, dict) or not answer.get("id") or not answer.get("secret"):
            raise ProviderError("paygate: create returned no key material", body=answer)
        return {
            "id": answer["id"],
            "secret": answer["secret"],
            "slot": spec.get("slot"),
            "created_at": answer.get("created_at"),
        }

    def revoke(self, spec: dict[str, Any], key_id: str, keep_until=None, **kwargs) -> dict[str, Any]:
        params: dict[str, Any] = {}
        if keep_until:
            params["effective_at"] = keep_until
        return self._request(
            "DELETE",
            f"/gateway/credentials/{key_id}",
            params=params,
            headers={"Authorization": f"Bearer {self.secret}"},
        )

    def describe(self, spec: dict[str, Any], key_id: str) -> dict[str, Any]:
        answer = self._request(
            "GET",
            f"/gateway/credentials/{key_id}",
            headers={"Authorization": f"Bearer {self.secret}"},
        )
        return {"state": (answer or {}).get("status", "unknown")}

    def probe(self, spec: dict[str, Any], key_id: str, secret: str, timeout: float = 10.0) -> bool:
        response = self.session.get(
            self._url(f"/gateway/credentials/{key_id}/verify"),
            headers={"Authorization": f"Bearer {secret}"},
            timeout=timeout,
        )
        return getattr(response, "status_code", 0) == 200
