"""Shared plumbing for the vendor adapters."""

from __future__ import annotations

import json
from typing import Any

try:  # pragma: no cover
    import requests
except Exception:  # pragma: no cover
    requests = None  # type: ignore[assignment]


DEFAULT_TIMEOUT = 10.0


class ProviderError(RuntimeError):
    """A vendor call did not produce a usable answer."""

    def __init__(self, message: str, *, status: int | None = None, body: Any = None):
        super().__init__(message)
        self.status = status
        self.body = body


class VerificationError(ProviderError):
    """A live pre-revocation probe did not confirm a credential."""


class Provider:
    """Base class with the HTTP helpers every adapter needs."""

    #: value used when a plan does not pin a timeout
    timeout = DEFAULT_TIMEOUT

    def __init__(self, name: str, config: dict[str, Any], session=None):
        self.name = name
        self.config = dict(config or {})
        self.endpoint = str(self.config.get("endpoint", "")).rstrip("/")
        self.secret = self.config.get("secret", "")
        if session is None:
            if requests is None:  # pragma: no cover
                raise RuntimeError("requests is not available")
            session = requests.Session()
        self.session = session

    # -- transport -----------------------------------------------------
    def _url(self, path: str) -> str:
        return f"{self.endpoint}{path}"

    def _request(self, method: str, path: str, **kwargs) -> Any:
        kwargs.setdefault("timeout", self.timeout)
        try:
            response = self.session.request(method, self._url(path), **kwargs)
        except Exception as exc:
            raise ProviderError(f"{self.name}: {method} {path} failed: {exc}") from exc
        status = getattr(response, "status_code", 0)
        if not 200 <= status < 300:
            raise ProviderError(
                f"{self.name}: {method} {path} returned {status}",
                status=status,
                body=getattr(response, "text", ""),
            )
        if status == 204:
            return {}
        try:
            return response.json()
        except Exception as exc:
            raise ProviderError(f"{self.name}: {method} {path} returned non-JSON") from exc

    # -- contract ------------------------------------------------------
    def create(self, spec: dict[str, Any]) -> dict[str, Any]:
        raise NotImplementedError

    def revoke(self, spec: dict[str, Any], key_id: str, keep_until=None, **kwargs) -> dict[str, Any]:
        raise NotImplementedError

    def describe(self, spec: dict[str, Any], key_id: str) -> dict[str, Any]:
        raise NotImplementedError

    def probe(self, spec: dict[str, Any], key_id: str, secret: str, timeout: float = DEFAULT_TIMEOUT) -> bool:
        raise NotImplementedError


def _json_body(payload: Any) -> str:
    return json.dumps(payload, sort_keys=True, separators=(",", ":"))


def build_providers(plan, *, session=None) -> dict[str, Provider]:
    """Instantiate one adapter per service declared in ``plan``."""
    from . import objstore, paygate, signer, smsgw

    registry = {
        "signer": signer.SignerProvider,
        "objstore": objstore.ObjStoreProvider,
        "paygate": paygate.PaygateProvider,
        "smsgw": smsgw.SmsGatewayProvider,
    }
    providers: dict[str, Provider] = {}
    for service, config in plan.services.items():
        cls = registry.get(service, Provider)
        providers[service] = cls(service, config, session=session)
    return providers
