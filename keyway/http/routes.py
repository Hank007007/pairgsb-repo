"""Request routing for keywaytransit.

The transport is deliberately thin: a request arrives as a method, a path,
a header map and a JSON body; a response leaves as a status code and a JSON
body.  ``dispatch`` is the single entry point both the server and the tests
use.
"""

from __future__ import annotations

import base64
import binascii
import time
from dataclasses import dataclass, field
from typing import Any, Mapping

from .. import keystore as keystore_mod
from ..errors import InvalidBlob, InvalidRequest, KeywayError
from ..format import MAX_KEY_NAME_LEN

TENANT_HEADER = "x-tenant-id"
MAX_BATCH_ITEMS = 64


@dataclass
class Request:
    method: str
    path: str
    headers: dict[str, str] = field(default_factory=dict)
    body: dict[str, Any] = field(default_factory=dict)

    @property
    def tenant_id(self) -> str:
        for name, value in self.headers.items():
            if name.lower() == TENANT_HEADER:
                if isinstance(value, str) and value:
                    return value
                return ""
        return ""


@dataclass
class Response:
    status: int
    body: dict[str, Any]


def _b64e(data: bytes) -> str:
    return base64.b64encode(data).decode("ascii")


def _b64d(text: str) -> bytes:
    try:
        return base64.b64decode(text, validate=True)
    except (binascii.Error, ValueError):
        raise InvalidBlob("blob is not valid base64")


def _context_from(body: dict[str, Any]):
    if "context" not in body:
        return None
    context = body["context"]
    if not isinstance(context, Mapping):
        raise InvalidRequest("context must be an object")
    for key, value in context.items():
        if not isinstance(key, str) or not isinstance(value, str):
            raise InvalidRequest("context keys and values must be strings")
    return dict(context)


def _require_key_name(body: dict[str, Any]) -> str:
    name = body.get("key_name")
    if not isinstance(name, str) or not name:
        raise InvalidRequest("key_name required")
    if len(name.encode("utf-8")) > MAX_KEY_NAME_LEN:
        raise InvalidRequest("key_name too long", key_name=name)
    return name


def _decode_blob_field(value: Any) -> bytes:
    if not isinstance(value, str) or not value:
        raise InvalidRequest("blob required")
    return _b64d(value)


def _audit_entry(request: Request, tenant_id: str, response: Response) -> dict:
    # §4.5: audit records identify who did what but never carry plaintext
    # or a wholesale copy of the request body.
    body = request.body if isinstance(request.body, dict) else {}
    entry = {
        "ts": time.time(),
        "tenant": tenant_id,
        "operation": request.path,
        "result": response.body.get("error", "ok"),
    }
    key_name = body.get("key_name")
    if isinstance(key_name, str):
        entry["key_name"] = key_name
    version = response.body.get("version")
    if isinstance(version, int):
        entry["version"] = version
    return entry


class Router:
    def __init__(self, store, audit=None):
        self.store = store
        self.audit = audit

    # -- dispatch --------------------------------------------------------

    def dispatch(self, request: Request) -> Response:
        tenant_id = request.tenant_id
        try:
            response = self._route(request, tenant_id)
        except KeywayError as exc:
            response = Response(
                status=exc.status,
                body=self._error_body(exc),
            )
        except Exception:  # noqa: BLE001 - never leak internals to callers
            response = Response(500, {"error": "internal_error"})
        if self.audit is not None:
            self.audit.record(_audit_entry(request, tenant_id, response))
        return response

    def _error_body(self, exc: KeywayError) -> dict[str, Any]:
        body: dict[str, Any] = {"error": exc.code}
        if exc.key_name is not None:
            body["key_name"] = exc.key_name
        return body

    def _route(self, request: Request, tenant_id: str) -> Response:
        if request.method != "POST":
            return Response(405, {"error": "method_not_allowed"})

        handler = {
            "/v1/encrypt": self._encrypt,
            "/v1/decrypt": self._decrypt,
            "/v1/batch": self._batch,
            "/v1/rewrap": self._rewrap,
            "/v1/rotate": self._rotate,
            "/v1/destroy": self._destroy,
        }.get(request.path)
        if handler is None:
            return Response(404, {"error": "not_found"})
        if not tenant_id:
            return Response(400, {"error": "invalid_request"})
        return handler(request, tenant_id)

    # -- handlers --------------------------------------------------------

    def _encrypt(self, request: Request, tenant_id: str) -> Response:
        body = request.body
        key_name = _require_key_name(body)
        plaintext = body.get("plaintext", "")
        if not isinstance(plaintext, str):
            raise InvalidRequest("plaintext must be a string")
        blob = self.store.encrypt(
            tenant_id, key_name, plaintext.encode("utf-8"), _context_from(body)
        )
        return Response(200, {"blob": _b64e(blob)})

    def _decrypt(self, request: Request, tenant_id: str) -> Response:
        body = request.body
        blob = _decode_blob_field(body.get("blob"))
        plaintext = self.store.decrypt(tenant_id, blob, _context_from(body))
        return Response(200, {"plaintext": plaintext.decode("utf-8", "replace")})

    def _batch(self, request: Request, tenant_id: str) -> Response:
        from .batch import handle_batch

        return handle_batch(self.store, tenant_id, request)

    def _rewrap(self, request: Request, tenant_id: str) -> Response:
        body = request.body
        blob = _decode_blob_field(body.get("blob"))
        out = self.store.rewrap(tenant_id, blob, _context_from(body))
        return Response(200, {"blob": _b64e(out)})

    def _rotate(self, request: Request, tenant_id: str) -> Response:
        key_name = _require_key_name(request.body)
        version = self.store.rotate(tenant_id, key_name)
        return Response(200, {"key_name": key_name, "version": version})

    def _destroy(self, request: Request, tenant_id: str) -> Response:
        body = request.body
        key_name = _require_key_name(body)
        version = body.get("version")
        if not isinstance(version, int) or version <= 0:
            raise InvalidRequest("version must be a positive integer")
        self.store.destroy_version(tenant_id, key_name, version)
        return Response(200, {"key_name": key_name, "version": version})


def dispatch(router: Router, request: Request) -> Response:
    return router.dispatch(request)
