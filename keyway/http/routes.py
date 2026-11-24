"""Request routing for keywaytransit.

The transport is deliberately thin: a request arrives as a method, a path, a
header map and a JSON body; a response leaves as a status code and a JSON
body.  ``dispatch`` is the single entry point both the server and the tests
use.
"""

from __future__ import annotations

import base64
import binascii
import traceback
from dataclasses import dataclass, field
from typing import Any

from .. import keystore as keystore_mod
from ..errors import InvalidBlob, InvalidRequest, KeywayError

TENANT_HEADER = "x-tenant-id"


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
                return value
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
    return body["context"]


def _require_key_name(body: dict[str, Any]) -> str:
    name = body.get("key_name")
    if not isinstance(name, str) or not name:
        raise InvalidRequest("key_name required")
    return name


class Router:
    def __init__(self, store: keystore_mod.KeyStore, audit=None):
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
        if self.audit is not None:
            self.audit.record(
                {
                    "tenant": tenant_id,
                    "method": request.method,
                    "path": request.path,
                    "body": request.body,
                },
                response.body,
            )
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
            return Response(400, {"error": "invalid_request"})
        blob = self.store.encrypt(
            tenant_id, key_name, plaintext.encode("utf-8"), _context_from(body)
        )
        return Response(200, {"blob": _b64e(blob)})

    def _decrypt(self, request: Request, tenant_id: str) -> Response:
        body = request.body
        try:
            blob = _b64d(body.get("blob", ""))
            plaintext = self.store.decrypt(tenant_id, blob, _context_from(body))
        except Exception as exc:  # noqa: BLE001 - reported to the caller below
            return Response(
                500,
                {
                    "error": "internal_error",
                    "detail": str(exc),
                    "trace": traceback.format_exc(),
                },
            )
        return Response(200, {"plaintext": plaintext.decode("utf-8", "replace")})

    def _batch(self, request: Request, tenant_id: str) -> Response:
        from .batch import handle_batch

        return Response(200, handle_batch(self.store, tenant_id, request.body))

    def _rewrap(self, request: Request, tenant_id: str) -> Response:
        body = request.body
        blob = _b64d(body.get("blob", ""))
        out = self.store.rewrap(tenant_id, blob, _context_from(body))
        return Response(200, {"blob": _b64e(out)})

    def _rotate(self, request: Request, tenant_id: str) -> Response:
        key_name = _require_key_name(request.body)
        version = self.store.rotate(tenant_id, key_name)
        return Response(200, {"key_name": key_name, "version": version})

    def _destroy(self, request: Request, tenant_id: str) -> Response:
        body = request.body
        key_name = _require_key_name(body)
        version = int(body.get("version", 0))
        self.store.destroy_version(tenant_id, key_name, version)
        return Response(200, {"key_name": key_name, "version": version})


def dispatch(router: Router, request: Request) -> Response:
    return router.dispatch(request)
