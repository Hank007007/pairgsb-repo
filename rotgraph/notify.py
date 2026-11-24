"""Deliver a freshly minted key to internal consumers and collect receipts.

The wire format and the retry rules live in ``docs/notify.md``.
"""

from __future__ import annotations

import hashlib
import hmac
import json
import secrets
import time
from dataclasses import dataclass, field
from typing import Any, Callable

try:  # pragma: no cover - import shape only
    import requests
except Exception:  # pragma: no cover
    requests = None  # type: ignore[assignment]


DEFAULT_TIMEOUT = 10.0


@dataclass
class Ack:
    """One consumer's answer to one push."""

    consumer: str
    ok: bool
    attempts: int = 1
    status: int | None = None
    detail: dict[str, Any] = field(default_factory=dict)


def sign(secret: str, timestamp: str, nonce: str, body: str) -> str:
    """Return the hex HMAC-SHA256 signature for one webhook request."""
    if isinstance(body, bytes):
        body = body.decode("utf-8")
    message = f"{timestamp}|{nonce}|{body}".encode("utf-8")
    return hmac.new(secret.encode("utf-8"), message, hashlib.sha256).hexdigest()


def _default_session():
    if requests is None:  # pragma: no cover
        raise RuntimeError("requests is not available")
    return requests.Session()


def _body_for(plan, spec, material: dict[str, Any]) -> str:
    payload = {
        "batch": plan.batch,
        "service": spec.service,
        "slot": spec.slot,
        "key_ref": material.get("id"),
        "key": material.get("secret"),
        "name": spec.name,
    }
    return json.dumps(payload, sort_keys=True, separators=(",", ":"))


def broadcast(
    plan,
    spec,
    material: dict[str, Any],
    *,
    session=None,
    sleep: Callable[[float], None] = time.sleep,
    timeout: float = DEFAULT_TIMEOUT,
) -> list[Ack]:
    """POST ``material`` to every consumer in the batch.

    Returns one :class:`Ack` per consumer describing how the push went.
    """
    if session is None:
        session = _default_session()

    body = _body_for(plan, spec, material)
    results: list[Ack] = []

    for consumer in plan.consumers:
        name = consumer["name"]
        url = consumer["endpoint"].rstrip("/") + "/webhook/rotate"
        secret = consumer.get("secret", "")
        attempts = 0
        ack = Ack(consumer=name, ok=False)

        while attempts < 3:
            attempts += 1
            timestamp = str(int(time.time()))
            nonce = secrets.token_hex(16)
            signature = hmac.new(
                secret.encode("utf-8"), body.encode("utf-8"), hashlib.sha256
            ).hexdigest()
            headers = {
                "Content-Type": "application/json",
                "X-Rot-Timestamp": timestamp,
                "X-Rot-Nonce": nonce,
                "X-Rot-Signature": signature,
            }
            try:
                response = session.post(url, data=body, headers=headers, timeout=timeout)
            except Exception as exc:  # connection reset, read timeout, ...
                ack = Ack(consumer=name, ok=False, attempts=attempts,
                          detail={"error": str(exc)})
                sleep(0.5 * attempts)
                continue

            status = getattr(response, "status_code", 0)
            ack = Ack(consumer=name, ok=200 <= status < 300, attempts=attempts, status=status)
            if status == 200:
                break
            sleep(0.5 * attempts)

        results.append(ack)

    return results


def await_acks(
    plan,
    spec,
    dispatched: list[Ack],
    *,
    session=None,
    timeout: float | None = None,
    poll_interval: float | None = None,
    clock: Callable[[], float] = time.time,
    sleep: Callable[[float], None] = time.sleep,
) -> list[Ack]:
    """Poll consumers for a receipt covering ``spec``.

    Returns the receipts that have arrived so far.
    """
    if session is None:
        session = _default_session()
    timeout = plan.notify.get("ack_timeout", 60.0) if timeout is None else timeout
    poll_interval = plan.notify.get("poll_interval", 1.0) if poll_interval is None else poll_interval

    pending = {ack.consumer: ack for ack in dispatched if ack.ok}
    received: list[Ack] = []
    deadline = clock() + timeout

    while pending and clock() < deadline:
        for name in list(pending):
            consumer = next(c for c in plan.consumers if c["name"] == name)
            url = consumer["endpoint"].rstrip("/") + "/webhook/receipt"
            params = {"batch": plan.batch, "key": spec.name}
            try:
                response = session.get(url, params=params, timeout=10.0)
                if getattr(response, "status_code", 0) != 200:
                    continue
                payload = response.json()
            except Exception:
                continue
            if isinstance(payload, dict) and payload.get("acked"):
                received.append(
                    Ack(
                        consumer=name,
                        ok=True,
                        status=200,
                        detail={"receipt": payload},
                    )
                )
                del pending[name]
        if not pending:
            break
        sleep(poll_interval)

    return received
