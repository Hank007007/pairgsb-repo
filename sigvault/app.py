"""HTTP entry points for the presigning service."""

from __future__ import annotations

import argparse
import json
import os
import time
from dataclasses import dataclass, field
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, quote, unquote, urlsplit

from . import policy, sign, store, verify

DEFAULT_EXPIRES_IN = 300
MAX_EXPIRES_IN = 3600
DEFAULT_CONTENT_TYPE = "application/octet-stream"


class AuthError(Exception):
    """Raised when a caller does not present valid service credentials."""


@dataclass(frozen=True)
class Service:
    """A business caller allowed to mint presigned URLs."""

    name: str
    token: str
    secret: bytes
    prefixes: tuple[str, ...]
    max_upload: int = 8 * 1024 * 1024


@dataclass
class Response:
    """A transport-independent response."""

    status: int
    body: bytes = b""
    headers: dict[str, str] = field(default_factory=dict)


def _json(status: int, payload: dict) -> Response:
    body = json.dumps(payload, ensure_ascii=False, sort_keys=True).encode("utf-8")
    return Response(status, body, {"Content-Type": "application/json; charset=utf-8"})


def _first(params: dict[str, list[str]], name: str, default: str | None = None) -> str | None:
    values = params.get(name)
    if not values:
        return default
    return values[0]


def media_type(value: str | None) -> str:
    """Reduce a Content-Type header to its lowercase media type."""
    if not value:
        return ""
    return value.split(";", 1)[0].strip().lower()


class SigVault:
    """Coordinates signing, verification, policy and storage over HTTP verbs."""

    def __init__(self, root: str | os.PathLike[str], services: list[Service], clock=time.time) -> None:
        self.root = os.fspath(root)
        self._services = {service.name: service for service in services}
        self._by_token = {service.token: service for service in services}
        self._clock = clock

    # -- request entry ---------------------------------------------------

    def handle(
        self,
        method: str,
        target: str,
        headers: dict[str, str] | None = None,
        body: bytes = b"",
    ) -> Response:
        lowered = {name.lower(): value for name, value in (headers or {}).items()}
        parts = urlsplit(target)
        try:
            if method == "POST" and parts.path == "/presign":
                return self._presign(lowered, body)
            if method in ("GET", "PUT"):
                return self._object(method, parts.path, parts.query, lowered, body)
            return _json(405, {"error": "method_not_allowed"})
        except AuthError as exc:
            return _json(401, {"error": "unauthorized", "detail": str(exc)})
        except policy.PolicyError as exc:
            return _json(403, {"error": "key_not_allowed", "detail": str(exc)})
        except verify.VerificationError as exc:
            return _json(403, {"error": "signature_invalid", "detail": str(exc)})
        except store.PayloadTooLarge as exc:
            return _json(413, {"error": "payload_too_large", "detail": str(exc)})
        except store.PathEscapeError as exc:
            return _json(400, {"error": "path_escape", "detail": str(exc)})
        except store.StoreError as exc:
            return _json(500, {"error": "storage_failure", "detail": str(exc)})
        except FileNotFoundError:
            return _json(404, {"error": "not_found"})
        except (KeyError, TypeError, ValueError) as exc:
            return _json(400, {"error": "bad_request", "detail": str(exc)})

    # -- presigning ------------------------------------------------------

    def _authenticate(self, headers: dict[str, str]) -> Service:
        raw = headers.get("authorization", "")
        scheme, _, token = raw.partition(" ")
        if scheme.lower() != "bearer" or not token:
            raise AuthError("a bearer token is required")
        service = self._by_token.get(token)
        if service is None:
            raise AuthError("unknown service credentials")
        return service

    def _presign(self, headers: dict[str, str], body: bytes) -> Response:
        service = self._authenticate(headers)
        payload = json.loads(body.decode("utf-8") or "{}")
        if not isinstance(payload, dict):
            raise ValueError("request body must be a JSON object")

        bucket = str(payload["bucket"])
        key = unquote(str(payload["key"]))
        target_method = str(payload.get("method", "GET")).upper()
        if target_method not in ("GET", "PUT"):
            raise ValueError("unsupported target method")

        expires_in = int(payload.get("expires_in", DEFAULT_EXPIRES_IN))
        if not 0 < expires_in <= MAX_EXPIRES_IN:
            raise ValueError("expires_in is out of range")

        if target_method == "GET":
            max_size = service.max_upload
            content_type = ""
        else:
            max_size = int(payload.get("max_size", service.max_upload))
            if max_size <= 0 or max_size > service.max_upload:
                raise ValueError("max_size is outside the service limit")
            content_type = media_type(str(payload.get("content_type") or DEFAULT_CONTENT_TYPE))

        policy.check_key(key, service.prefixes)

        exp = int(self._clock()) + expires_in
        signature = sign.sign(
            service.secret,
            target_method,
            bucket,
            key,
            content_type,
            max_size,
            exp,
        )

        params = [("svc", service.name), ("exp", str(exp)), ("sig", signature)]
        if target_method == "PUT":
            params.append(("max-size", str(max_size)))
        query = "&".join(f"{name}={value}" for name, value in params)
        url = f"/{bucket}/{quote(key, safe='/')}?{query}"

        return _json(
            200,
            {
                "url": url,
                "method": target_method,
                "expires_at": exp,
                "content_type": content_type,
                "max_size": max_size,
            },
        )

    # -- object access ---------------------------------------------------

    def _object(
        self,
        method: str,
        path: str,
        query: str,
        headers: dict[str, str],
        body: bytes,
    ) -> Response:
        params = parse_qs(query, keep_blank_values=True)
        relative = path.lstrip("/")
        if not relative or "/" not in relative:
            raise ValueError("object path must be /<bucket>/<key>")
        bucket, key = relative.split("/", 1)
        key = unquote(key)

        service = self._services.get(_first(params, "svc") or "")
        if service is None:
            raise verify.VerificationError("unknown service")

        max_size = int(_first(params, "max-size", str(service.max_upload)))
        if method == "PUT":
            content_type = media_type(headers.get("content-type") or DEFAULT_CONTENT_TYPE)
        else:
            content_type = ""

        verify.verify(
            service.secret,
            method=method,
            bucket=bucket,
            key=key,
            content_type=content_type,
            max_size=max_size,
            exp=_first(params, "exp"),
            signature=_first(params, "sig") or "",
            now=self._clock(),
        )

        normalized = policy.check_key(key, service.prefixes)

        if method == "PUT":
            store.receive(self.root, bucket, normalized, body, max_size)
            return _json(201, {"stored": f"{bucket}/{normalized}", "size": len(body)})

        data = store.read(self.root, bucket, normalized)
        return Response(200, data, {"Content-Type": "application/octet-stream"})


def parse_service(spec: str) -> Service:
    """Parse ``NAME=SECRET:TOKEN:PREFIX[,PREFIX]:MAX_BYTES``."""
    name, _, rest = spec.partition("=")
    secret, token, prefixes, max_bytes = rest.rsplit(":", 3)
    return Service(
        name=name,
        token=token,
        secret=secret.encode("utf-8"),
        prefixes=tuple(part for part in prefixes.split(",") if part),
        max_upload=int(max_bytes),
    )


def make_server(host: str, port: int, vault: SigVault) -> ThreadingHTTPServer:
    """Wrap a :class:`SigVault` in a threaded HTTP server."""

    class Handler(BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        def _dispatch(self) -> None:
            length = int(self.headers.get("Content-Length") or 0)
            body = self.rfile.read(length) if length else b""
            response = vault.handle(self.command, self.path, dict(self.headers.items()), body)
            self.send_response(response.status)
            for name, value in response.headers.items():
                self.send_header(name, value)
            self.send_header("Content-Length", str(len(response.body)))
            self.end_headers()
            if response.body:
                self.wfile.write(response.body)

        do_GET = _dispatch
        do_PUT = _dispatch
        do_POST = _dispatch

        def log_message(self, format: str, *args: object) -> None:
            pass

    return ThreadingHTTPServer((host, port), Handler)


def main(argv: list[str] | None = None) -> None:
    parser = argparse.ArgumentParser(prog="sigvault")
    parser.add_argument("--root", required=True, help="object storage root directory")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=8080)
    parser.add_argument(
        "--service",
        action="append",
        default=[],
        metavar="NAME=SECRET:TOKEN:PREFIX[,PREFIX]:MAX_BYTES",
        help="register a business caller (repeatable)",
    )
    args = parser.parse_args(argv)

    vault = SigVault(args.root, [parse_service(spec) for spec in args.service])
    server = make_server(args.host, args.port, vault)
    host, port = server.server_address[:2]
    print(f"sigvault listening on http://{host}:{port} (root={vault.root})")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
