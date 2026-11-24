"""Drive one rotation batch.

The end-to-end flow is described in ``docs/rotation-plan.md``,
``docs/swap-window.md``, ``docs/notify.md`` and ``docs/recovery.md``.
"""

from __future__ import annotations

import time
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any, Callable

from . import journal, notify, probe
from .plan import KeySpec, Plan, PlanError, load_plan
from .providers import build_providers
from .providers.base import ProviderError


PHASE_PENDING = "pending"
PHASE_CREATED = "created"
PHASE_NOTIFIED = "notified"
PHASE_DWELL = "dwell"
PHASE_REVOKED = "revoked"
PHASE_ROLLED_BACK = "rolled_back"


def _iso(moment: datetime) -> str:
    return (
        moment.astimezone(timezone.utc)
        .replace(microsecond=0)
        .isoformat()
        .replace("+00:00", "Z")
    )


@dataclass
class KeyOutcome:
    key: str
    phase: str = PHASE_PENDING
    created_at: float | None = None
    new_key_id: str | None = None
    old_key_id: str | None = None
    acks: list[notify.Ack] = field(default_factory=list)
    note: str = ""

    @property
    def revoke_failed(self) -> bool:
        return self.note.startswith("revoke_failed")


@dataclass
class BatchResult:
    batch: str
    ok: bool
    outcomes: dict[str, KeyOutcome] = field(default_factory=dict)
    error: str = ""

    def to_dict(self) -> dict[str, Any]:
        return {
            "batch": self.batch,
            "ok": self.ok,
            "error": self.error,
            "keys": {
                name: {
                    "phase": outcome.phase,
                    "new_key_ref": outcome.new_key_id,
                    "note": outcome.note,
                }
                for name, outcome in self.outcomes.items()
            },
        }


class Rotator:
    """State shared by :func:`execute`, :func:`advance` and :func:`rollback`."""

    def __init__(
        self,
        plan: Plan,
        *,
        providers=None,
        journal_path: str | None = None,
        session=None,
        clock: Callable[[], float] = time.time,
        sleep: Callable[[float], None] = time.sleep,
    ):
        self.plan = plan
        self.providers = (
            providers if providers is not None else build_providers(plan, session=session)
        )
        self.journal_path = journal_path
        self.session = session
        self.clock = clock
        self.sleep = sleep
        self.outcomes: dict[str, KeyOutcome] = {}
        self.material: dict[str, dict[str, Any]] = {}

    # -- journal ---------------------------------------------------------
    def _write(self, spec: KeySpec, action: str, phase: str, detail: dict[str, Any]) -> None:
        if not self.journal_path:
            return
        record = journal.make_record(self.plan.batch, spec.name, action, phase, detail)
        journal.append(self.journal_path, record)

    def _outcome(self, spec: KeySpec) -> KeyOutcome:
        if spec.name not in self.outcomes:
            self.outcomes[spec.name] = KeyOutcome(
                key=spec.name,
                old_key_id=(spec.current or {}).get("id"),
            )
        return self.outcomes[spec.name]

    # -- steps -----------------------------------------------------------
    def _create(self, spec: KeySpec) -> dict[str, Any]:
        outcome = self._outcome(spec)
        provider = self.providers[spec.service]
        material = provider.create(spec.as_request())
        self.material[spec.name] = material
        outcome.new_key_id = material.get("id")
        outcome.created_at = self.clock()
        outcome.phase = PHASE_CREATED
        self._write(
            spec,
            "create",
            PHASE_CREATED,
            {
                "new_key_ref": material.get("id"),
                "new_secret": material.get("secret"),
                "old_key_ref": outcome.old_key_id,
            },
        )
        return material

    def _notify(self, spec: KeySpec, material: dict[str, Any]) -> list[notify.Ack]:
        outcome = self._outcome(spec)
        dispatched = notify.broadcast(
            self.plan,
            spec,
            material,
            session=self.session,
            sleep=self.sleep,
        )
        acks = notify.await_acks(
            self.plan,
            spec,
            dispatched,
            session=self.session,
            sleep=self.sleep,
            clock=self.clock,
        )
        outcome.acks = acks
        if acks:
            outcome.phase = PHASE_NOTIFIED
            self._write(
                spec,
                "notify",
                PHASE_NOTIFIED,
                {"new_key_ref": material.get("id"), "acked": len(acks)},
            )
        return acks

    def _check_before_revoke(self, spec: KeySpec) -> bool:
        outcome = self._outcome(spec)
        provider = self.providers[spec.service]
        ok = probe.verify(spec, self.material.get(spec.name), provider=provider)
        self._write(
            spec,
            "verify",
            PHASE_NOTIFIED,
            {"new_key_ref": outcome.new_key_id, "old_key_ref": outcome.old_key_id, "ok": ok},
        )
        return ok

    def _wait_for_dwell(self, spec: KeySpec) -> None:
        outcome = self._outcome(spec)
        if outcome.created_at is None:
            return
        remaining = self.plan.dwell - (self.clock() - outcome.created_at)
        if remaining > 0:
            self.sleep(remaining)
        outcome.phase = PHASE_DWELL
        self._write(spec, "verify", PHASE_DWELL, {"new_key_ref": outcome.new_key_id})

    def _revoke_old(self, spec: KeySpec) -> None:
        outcome = self._outcome(spec)
        if not outcome.old_key_id:
            outcome.phase = PHASE_REVOKED
            self._write(spec, "revoke", PHASE_REVOKED, {"old_key_ref": None})
            return

        provider = self.providers[spec.service]
        created = outcome.created_at if outcome.created_at is not None else self.clock()
        keep_until = _iso(datetime.fromtimestamp(created + self.plan.dwell, tz=timezone.utc))

        try:
            provider.revoke(spec.as_request(), outcome.old_key_id, keep_until=keep_until)
        except ProviderError as exc:
            outcome.note = f"revoke_failed: {exc}"
            self._write(
                spec,
                "revoke",
                PHASE_DWELL,
                {"old_key_ref": outcome.old_key_id, "error": str(exc)},
            )
            raise

        outcome.phase = PHASE_REVOKED
        self._write(
            spec,
            "revoke",
            PHASE_REVOKED,
            {"old_key_ref": outcome.old_key_id, "keep_until": keep_until},
        )


def advance(rotator: Rotator, spec: KeySpec) -> KeyOutcome:
    """Move one key forward: create, notify, hold the swap window, revoke."""
    outcome = rotator._outcome(spec)

    material = rotator._create(spec)
    rotator._notify(spec, material)
    rotator._check_before_revoke(spec)
    rotator._wait_for_dwell(spec)
    rotator._revoke_old(spec)
    return outcome


def rollback(rotator: Rotator) -> None:
    """Recover the keys this batch touched."""
    for spec in reversed(list(rotator.plan.keys)):
        outcome = rotator.outcomes.get(spec.name)
        if outcome is None:
            continue
        if not outcome.new_key_id:
            continue
        provider = rotator.providers[spec.service]
        rotator._write(
            spec,
            "rollback",
            PHASE_ROLLED_BACK,
            {"new_key_ref": outcome.new_key_id, "old_key_ref": outcome.old_key_id},
        )
        try:
            provider.revoke(spec.as_request(), outcome.new_key_id)
        except ProviderError as exc:
            outcome.note = f"rollback_failed: {exc}"
            continue
        outcome.phase = PHASE_ROLLED_BACK


def execute(
    plan_or_path,
    *,
    journal_path: str | None = None,
    providers=None,
    session=None,
    clock: Callable[[], float] = time.time,
    sleep: Callable[[float], None] = time.sleep,
) -> BatchResult:
    """Run one whole batch and return what happened to each key."""
    if isinstance(plan_or_path, Plan):
        plan = plan_or_path
    else:
        plan = load_plan(plan_or_path)

    rotator = Rotator(
        plan,
        providers=providers,
        journal_path=journal_path,
        session=session,
        clock=clock,
        sleep=sleep,
    )

    result = BatchResult(batch=plan.batch, ok=False)
    try:
        for spec in plan.keys:
            advance(rotator, spec)
    except (ProviderError, PlanError) as exc:
        result.error = str(exc)
        rollback(rotator)
        result.outcomes = dict(rotator.outcomes)
        return result

    result.outcomes = dict(rotator.outcomes)
    result.ok = all(o.phase == PHASE_REVOKED for o in result.outcomes.values())
    return result


def resume_batch(
    plan: Plan,
    journal_path: str,
    *,
    providers=None,
    session=None,
    clock: Callable[[], float] = time.time,
    sleep: Callable[[float], None] = time.sleep,
) -> BatchResult:
    """Continue a batch that already has journal records on disk."""
    state = journal.resume(journal_path)
    rotator = Rotator(
        plan,
        providers=providers,
        journal_path=journal_path,
        session=session,
        clock=clock,
        sleep=sleep,
    )
    for name in state.done:
        rotator.outcomes[name] = KeyOutcome(key=name, phase=PHASE_REVOKED)

    result = BatchResult(batch=plan.batch, ok=False)
    try:
        for spec in plan.keys:
            if spec.name in state.done:
                continue
            advance(rotator, spec)
    except (ProviderError, PlanError) as exc:
        result.error = str(exc)
        rollback(rotator)
        result.outcomes = dict(rotator.outcomes)
        return result

    result.outcomes = dict(rotator.outcomes)
    result.ok = all(o.phase == PHASE_REVOKED for o in result.outcomes.values())
    return result
