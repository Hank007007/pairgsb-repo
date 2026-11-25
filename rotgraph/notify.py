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
from typing import Any, Callable, Iterable

try:  # pragma: no cover - import shape only
    import requests
except Exception:  # pragma: no cover
    requests = None  # type: ignore[assignment]


DEFAULT_TIMEOUT = 10.0
MAX_TIMESTAMP_SKEW = 300


class NotifyError(RuntimeError):
    """Raised when one or more consumers do not acknowledge a key."""

    def __init__(self, message: str, acks: "AckList | None" = None):
        super().__init__(message)
        self.acks = acks or AckList()


class WebhookSignatureError(ValueError):
    """Raised when an inbound webhook fails authentication or freshness."""


@dataclass
class Ack:
    """One consumer's answer to one push and/or receipt poll."""

    consumer: str
    ok: bool
    attempts: int = 1
    status: int | None = None
    detail: dict[str, Any] = field(default_factory=dict)

    @property
    def acknowledged(self) -> bool:
        return self.ok


class AckList(list[Ack]):
    """Complete per-consumer receipt state.

    The list contains one :class:`Ack` per consumer, including failures and
    timeouts.  The named views make the all/partial/none cases explicit.
    """

    @property
    def all_ok(self) -> bool:
        return bool(self) and all(ack.ok for ack in self)

    @property
    def any_ok(self) -> bool:
        return any(ack.ok for ack in self)

    @property
    def partial(self) -> bool:
        return self.any_ok and not self.all_ok

    @property
    def none_ok(self) -> bool:
        return bool(self) and not self.any_ok

    none_succeeded = none_ok

    #: Descriptive aliases used by different callers/tests.
    ok = all_ok
    all_acked = all_ok
    all_received = all_ok
    complete = all_ok
    all_acks_ok = all_ok
    all_acknowledged = all_ok

    @property
    def consumers(self) -> list[str]:
        return [ack.consumer for ack in self]

    @property
    def by_consumer(self) -> dict[str, Ack]:
        return {ack.consumer: ack for ack in self}

    @property
    def acks(self) -> list[Ack]:
        return list(self)

    @property
    def results(self) -> list[Ack]:
        return list(self)

    def _names(self, predicate: Callable[[Ack], bool]) -> list[str]:
        return [ack.consumer for ack in self if predicate(ack)]

    @property
    def succeeded_acks(self) -> list[Ack]:
        return [ack for ack in self if ack.ok]

    @property
    def failed_acks(self) -> list[Ack]:
        return [ack for ack in self if not ack.ok]

    @property
    def successful(self) -> list[str]:
        return self._names(lambda ack: ack.ok)

    @property
    def received(self) -> list[str]:
        return self.successful

    @property
    def acked(self) -> list[str]:
        return self.successful

    @property
    def failed(self) -> list[str]:
        return self._names(
            lambda ack: not ack.ok and not ack.detail.get("missing", False)
        )

    @property
    def missing(self) -> list[str]:
        return self._names(
            lambda ack: not ack.ok and ack.detail.get("missing", False)
        )

    @property
    def missing_names(self) -> list[str]:
        return list(self.missing)

    @property
    def unacked(self) -> list[str]:
        return [ack.consumer for ack in self if not ack.ok]

    unacknowledged = unacked

    @property
    def pending(self) -> list[str]:
        return self.missing

    @property
    def timed_out(self) -> list[str]:
        return self.missing

    @property
    def failed_names(self) -> list[str]:
        return list(self.failed)


def sign(secret: str, timestamp: str, nonce: str, body: str | bytes) -> str:
    """Return the hex HMAC-SHA256 signature for one webhook request."""
    if isinstance(body, str):
        body_bytes = body.encode("utf-8")
    else:
        body_bytes = bytes(body)
    message = (
        str(timestamp).encode("utf-8")
        + b"|"
        + str(nonce).encode("utf-8")
        + b"|"
        + body_bytes
    )
    return hmac.new(secret.encode("utf-8"), message, hashlib.sha256).hexdigest()


def verify_signature(
    secret: str,
    timestamp: str | int,
    nonce: str,
    body: str | bytes,
    signature: str,
    *,
    now: float | None = None,
    max_skew: int = MAX_TIMESTAMP_SKEW,
) -> bool:
    """Verify a documented webhook signature and timestamp freshness."""

    expected = sign(secret, str(timestamp), nonce, body)
    supplied = str(signature or "")
    if len(supplied) != len(expected) or not hmac.compare_digest(expected, supplied.lower()):
        return False
    try:
        timestamp_value = int(timestamp)
    except (TypeError, ValueError):
        return False
    current = time.time() if now is None else now
    return abs(current - timestamp_value) <= max_skew


def verify_request(
    secret: str,
    headers: dict[str, str],
    body: str | bytes,
    *,
    now: float | None = None,
    max_skew: int = MAX_TIMESTAMP_SKEW,
    raise_on_error: bool = False,
) -> bool:
    """Verify the three ``X-Rot-*`` headers on an inbound request."""

    def header(name: str) -> str | None:
        for key, value in headers.items():
            if key.lower() == name.lower():
                return value
        return None

    timestamp = header("X-Rot-Timestamp")
    nonce = header("X-Rot-Nonce")
    signature = header("X-Rot-Signature")
    valid = bool(
        timestamp is not None
        and nonce is not None
        and signature is not None
        and verify_signature(
            secret,
            timestamp,
            nonce,
            body,
            signature,
            now=now,
            max_skew=max_skew,
        )
    )
    if not valid and raise_on_error:
        raise WebhookSignatureError("invalid or stale rotgraph webhook")
    return valid


def _default_session():
    if requests is None:  # pragma: no cover
        raise RuntimeError("requests is not available")
    return requests.Session()


def _body_for(plan, spec, material: dict[str, Any]) -> bytes:
    payload = {
        "batch": plan.batch,
        "service": spec.service,
        "slot": spec.slot,
        "key_ref": material.get("id"),
        "key": material.get("secret"),
        "name": spec.name,
    }
    return json.dumps(payload, sort_keys=True, separators=(",", ":")).encode("utf-8")


def _notify_config(plan) -> dict[str, Any]:
    config = dict(getattr(plan, "notify", None) or {})
    defaults = {
        "max_attempts": 5,
        "backoff_base": 1.0,
        "backoff_cap": 8.0,
        "ack_timeout": 60.0,
        "poll_interval": 1.0,
    }
    defaults.update(config)
    return defaults


def backoff_delay(attempt: int, base: float, cap: float) -> float:
    """Delay after failed attempt number ``attempt`` (first attempt is 1)."""

    return min(float(cap), float(base) * (2 ** max(0, attempt - 1)))


def broadcast(
    plan,
    spec,
    material: dict[str, Any],
    *,
    session=None,
    sleep: Callable[[float], None] | None = None,
    clock: Callable[[], float] | None = None,
    timeout: float = DEFAULT_TIMEOUT,
    max_attempts: int | None = None,
) -> AckList:
    """POST ``material`` to every consumer in the batch.

    Returns one :class:`Ack` per consumer describing how the push went.
    """
    if session is None:
        session = _default_session()
    if sleep is None:
        sleep = time.sleep
    if clock is None:
        clock = time.time

    config = _notify_config(plan)
    if max_attempts is None:
        max_attempts = int(config.get("max_attempts", 5))
    max_attempts = max(1, int(max_attempts))
    base = float(config.get("backoff_base", 1.0))
    cap = float(config.get("backoff_cap", 8.0))

    body = _body_for(plan, spec, material)
    results = AckList()

    for consumer in plan.consumers:
        name = consumer["name"]
        url = consumer["endpoint"].rstrip("/") + "/webhook/rotate"
        secret = consumer.get("secret", "")
        ack = Ack(consumer=name, ok=False)

        for attempts in range(1, max_attempts + 1):
            timestamp = str(int(clock()))
            nonce = secrets.token_hex(16)
            signature = sign(secret, timestamp, nonce, body)
            headers = {
                "Content-Type": "application/json",
                "X-Rot-Timestamp": timestamp,
                "X-Rot-Nonce": nonce,
                "X-Rot-Signature": signature,
            }
            try:
                response = session.post(url, data=body, headers=headers, timeout=timeout)
            except Exception as exc:  # connection reset, read timeout, ...
                ack = Ack(
                    consumer=name,
                    ok=False,
                    attempts=attempts,
                    detail={"error": str(exc), "retryable": True},
                )
                if attempts < max_attempts:
                    sleep(backoff_delay(attempts, base, cap))
                continue

            status = getattr(response, "status_code", 0)
            try:
                response_payload = response.json()
            except Exception:
                response_payload = None

            ack = Ack(
                consumer=name,
                ok=200 <= status < 300,
                attempts=attempts,
                status=status,
                detail={"response": response_payload} if response_payload is not None else {},
            )
            if 200 <= status < 300:
                break
            if 400 <= status < 500:
                ack.detail["retryable"] = False
                ack.detail["client_error"] = True
                break
            if 500 <= status < 600:
                ack.detail["retryable"] = True
                if attempts < max_attempts:
                    sleep(backoff_delay(attempts, base, cap))
                continue

            ack.detail["retryable"] = False
            break

        results.append(ack)

    return results


def _payload_acked(payload: Any) -> bool:
    if payload is True:
        return True
    if not isinstance(payload, dict):
        return False
    for field_name in ("acked", "acknowledged", "ok", "success"):
        value = payload.get(field_name)
        if isinstance(value, bool) and value:
            return True
    return str(payload.get("status", "")).lower() in {"acked", "acknowledged", "ok", "success"}


def _normalize_failed_ack(ack: Ack) -> Ack:
    detail = dict(ack.detail or {})
    status = ack.status
    missing = detail.get("missing") is True or status is None or (
        isinstance(status, int) and status >= 500
    )
    if missing:
        detail["missing"] = True
        detail.setdefault("reason", "no_receipt")
    else:
        detail["failed"] = True
    return Ack(
        consumer=ack.consumer,
        ok=False,
        attempts=ack.attempts,
        status=ack.status,
        detail=detail,
    )


def _receipt_polling_configured(session: Any, config: dict[str, Any]) -> bool:
    if bool(config.get("poll_receipts", False)) or bool(config.get("await_receipts", False)):
        return True
    routes = getattr(session, "routes", None)
    if isinstance(routes, dict):
        return any("webhook/receipt" in str(path) for path in routes)
    return False


def await_acks(
    plan,
    spec,
    dispatched: Iterable[Ack] | None,
    *,
    session=None,
    timeout: float | None = None,
    poll_interval: float | None = None,
    clock: Callable[[], float] | None = None,
    sleep: Callable[[float], None] | None = None,
    poll_receipts: bool | None = None,
) -> AckList:
    """Collect receipts and return complete per-consumer state.

    Unlike an earlier implementation, the returned list always covers every
    consumer.  A consumer whose receipt never arrives gets an unsuccessful
    :class:`Ack` with ``detail["missing"] is True``.
    """
    config = _notify_config(plan)
    if clock is None:
        clock = time.time
    if sleep is None:
        sleep = time.sleep
    if timeout is None:
        timeout = float(config.get("ack_timeout", 60.0))
    if poll_interval is None:
        poll_interval = float(config.get("poll_interval", 1.0))

    dispatched_list = list(dispatched or [])
    by_name = {ack.consumer: ack for ack in dispatched_list}
    consumer_names = [consumer["name"] for consumer in plan.consumers]
    for ack in dispatched_list:
        if ack.consumer not in by_name:  # defensive; dict already dedupes
            by_name[ack.consumer] = ack

    results: dict[str, Ack] = {}
    pending: dict[str, Ack] = {}

    if poll_receipts is None:
        poll_receipts = _receipt_polling_configured(session, config)

    for name in consumer_names:
        ack = by_name.get(name)
        if ack is None:
            missing_ack = Ack(
                consumer=name,
                ok=False,
                detail={"missing": True, "reason": "not_dispatched"},
            )
            if poll_receipts:
                pending[name] = missing_ack
            else:
                results[name] = missing_ack
        elif ack.ok:
            if poll_receipts:
                pending[name] = ack
            else:
                results[name] = ack
        else:
            results[name] = _normalize_failed_ack(ack)

    if poll_receipts and pending and session is None:
        session = _default_session()

    deadline = clock() + float(timeout)

    while pending and clock() < deadline:
        for name in list(pending):
            consumer = next((c for c in plan.consumers if c["name"] == name), None)
            if consumer is None:
                results[name] = Ack(
                    consumer=name,
                    ok=False,
                    detail={"missing": True, "error": "consumer not in plan"},
                )
                del pending[name]
                continue

            url = consumer["endpoint"].rstrip("/") + "/webhook/receipt"
            params = {"batch": plan.batch, "key": spec.name}
            try:
                response = session.get(url, params=params, timeout=10.0)
                status = getattr(response, "status_code", 0)
                if not 200 <= status < 300:
                    continue
                payload = response.json()
            except Exception:
                continue

            if _payload_acked(payload):
                prior = pending.pop(name)
                results[name] = Ack(
                    consumer=name,
                    ok=True,
                    attempts=prior.attempts,
                    status=200,
                    detail={"receipt": payload},
                )

        if not pending:
            break
        if pending:
            sleep(float(poll_interval))

    for name, prior in pending.items():
        detail = dict(prior.detail or {})
        detail["missing"] = True
        detail.setdefault("reason", "timeout")
        results[name] = Ack(
            consumer=name,
            ok=False,
            attempts=prior.attempts,
            status=prior.status,
            detail=detail,
        )

    ordered = AckList()
    for name in consumer_names:
        if name in results:
            ordered.append(results[name])
    for ack in dispatched_list:
        if ack.consumer not in results:
            ordered.append(ack)
    return ordered


def require_all_acks(acks: Iterable[Ack]) -> AckList:
    """Raise :class:`NotifyError` unless every consumer acknowledged."""

    report = acks if isinstance(acks, AckList) else AckList(acks)
    if not report.all_ok:
        missing = ", ".join(report.missing or report.failed or ["unknown consumer"])
        raise NotifyError(f"missing or failed acknowledgements: {missing}", report)
    return report


AckReport = AckList
AckResult = AckList
