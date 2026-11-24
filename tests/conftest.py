"""Shared fixtures: a recorded HTTP session and a in-memory vendor."""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from typing import Any


class FakeResponse:
    def __init__(self, status_code: int = 200, payload: Any = None, text: str | None = None):
        self.status_code = status_code
        self._payload = payload if payload is not None else {}
        self.text = text if text is not None else json.dumps(self._payload)

    def json(self):
        if isinstance(self._payload, str):
            return json.loads(self._payload)
        return self._payload


@dataclass
class Call:
    method: str
    url: str
    kwargs: dict[str, Any] = field(default_factory=dict)


class FakeSession:
    """Stand-in for ``requests.Session`` that records every call.

    ``routes`` maps a URL suffix (path after the host) to a list of responses
    that are consumed in order; the last one repeats once the list runs out.
    """

    def __init__(self, routes: dict[str, list[FakeResponse]] | None = None):
        self.routes = {k: list(v) for k, v in (routes or {}).items()}
        self.calls: list[Call] = []

    # -- plumbing ------------------------------------------------------
    @staticmethod
    def _path(url: str) -> str:
        for prefix in ("https://", "http://"):
            if url.startswith(prefix):
                rest = url[len(prefix):]
                slash = rest.find("/")
                return rest[slash:] if slash != -1 else "/"
        return url

    def _answer(self, method: str, url: str, **kwargs) -> FakeResponse:
        self.calls.append(Call(method=method, url=url, kwargs=kwargs))
        path = self._path(url)
        queue = self.routes.get(path)
        if not queue:
            return FakeResponse(200, {})
        if len(queue) > 1:
            return queue.pop(0)
        return queue[0]

    # -- requests.Session surface --------------------------------------
    def request(self, method: str, url: str, **kwargs):
        return self._answer(method.upper(), url, **kwargs)

    def post(self, url: str, **kwargs):
        return self._answer("POST", url, **kwargs)

    def get(self, url: str, **kwargs):
        return self._answer("GET", url, **kwargs)

    def head(self, url: str, **kwargs):
        return self._answer("HEAD", url, **kwargs)

    def delete(self, url: str, **kwargs):
        return self._answer("DELETE", url, **kwargs)

    def close(self):
        return None


class FakeProvider:
    """Minimal vendor used by tests that want to stay off the wire."""

    def __init__(self, name: str = "signer"):
        self.name = name
        self.created: list[dict[str, Any]] = []
        self.revoked: list[dict[str, Any]] = []
        self.counter = 0

    def create(self, spec):
        self.counter += 1
        material = {
            "id": f"{self.name}_new_{self.counter}",
            "secret": f"{self.name}-secret-{self.counter}",
            "slot": spec.get("slot"),
        }
        self.created.append(material)
        return material

    def revoke(self, spec, key_id, keep_until=None, **kwargs):
        self.revoked.append({"id": key_id, "keep_until": keep_until})
        return {"ok": True, "id": key_id}

    def describe(self, spec, key_id):
        return {"state": "active"}

    def probe(self, spec, key_id, secret, timeout=10.0):
        return True
