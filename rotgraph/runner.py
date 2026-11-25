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
from .plan import KeySpec, Plan, PlanError, load_plan, order
from .providers import build_providers
from .providers.base import ProviderError, VerificationError


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


def _coerce_providers(plan: Plan, providers: Any) -> dict[str, Any]:
    if isinstance(providers, dict):
        return providers
    if hasattr(providers, "create") and callable(getattr(providers, "create")):
        return {service: providers for service in plan.services}
    if isinstance(providers, (list, tuple)):
        mapping: dict[str, Any] = {}
        for provider in providers:
            name = getattr(provider, "name", None)
            if name:
                mapping[str(name)] = provider
        return mapping
    return dict(providers)


@dataclass
class KeyOutcome:
    key: str
    phase: str = PHASE_PENDING
    created_at: float | None = None
    new_key_id: str | None = None
    old_key_id: str | None = None
    acks: list[notify.Ack] = field(default_factory=list)
    note: str = ""
    needs_manual: bool = False
    probes: dict[str, bool] = field(default_factory=dict)
    notify_complete: bool = False
    create_confirmed: bool = False
    revoke_complete: bool = False
    rollback_complete: bool = False
    rollback_requested: bool = False

    @property
    def revoke_failed(self) -> bool:
        return self.note.startswith("revoke_failed")

    @property
    def manual(self) -> bool:
        return self.needs_manual

    @manual.setter
    def manual(self, value: bool) -> None:
        self.needs_manual = bool(value)

    @property
    def requires_manual(self) -> bool:
        return self.needs_manual

    @requires_manual.setter
    def requires_manual(self, value: bool) -> None:
        self.needs_manual = bool(value)

    @property
    def manual_intervention(self) -> bool:
        return self.needs_manual

    @manual_intervention.setter
    def manual_intervention(self, value: bool) -> None:
        self.needs_manual = bool(value)


@dataclass
class BatchResult:
    batch: str
    ok: bool
    outcomes: dict[str, KeyOutcome] = field(default_factory=dict)
    error: str = ""
    error_code: str | None = None
    error_kind: str | None = None
    resumed: bool = False
    diagnostics: list[str] = field(default_factory=list)
    manual_intervention: list[str] = field(default_factory=list)
    rolled_back: list[str] = field(default_factory=list)

    @property
    def needs_manual(self) -> list[str]:
        return list(self.manual_intervention)

    @property
    def reason(self) -> str | None:
        return self.error_code

    @property
    def code(self) -> str | None:
        return self.error_code

    @property
    def manual(self) -> list[str]:
        return list(self.manual_intervention)

    @property
    def needs_human(self) -> list[str]:
        return list(self.manual_intervention)

    @property
    def manual_review(self) -> list[str]:
        return list(self.manual_intervention)

    @property
    def failed_keys(self) -> list[str]:
        return [
            name
            for name, outcome in self.outcomes.items()
            if outcome.phase != PHASE_REVOKED or outcome.needs_manual
        ]

    def to_dict(self) -> dict[str, Any]:
        return {
            "batch": self.batch,
            "ok": self.ok,
            "error": self.error,
            "error_code": self.error_code,
            "error_kind": self.error_kind,
            "reason": self.error_code,
            "resumed": self.resumed,
            "diagnostics": list(self.diagnostics),
            "manual_intervention": list(self.manual_intervention),
            "needs_manual": list(self.manual_intervention),
            "manual": list(self.manual_intervention),
            "manual_keys": list(self.manual_intervention),
            "rolled_back": list(self.rolled_back),
            "failed_keys": self.failed_keys,
            "keys": {
                name: {
                    "phase": outcome.phase,
                    "new_key_ref": outcome.new_key_id,
                    "new_key_id": outcome.new_key_id,
                    "old_key_ref": outcome.old_key_id,
                    "old_key_id": outcome.old_key_id,
                    "note": outcome.note,
                    "needs_manual": outcome.needs_manual,
                    "manual_required": outcome.needs_manual,
                    "probes": dict(outcome.probes),
                    "acks": [
                        {
                            "consumer": ack.consumer,
                            "ok": ack.ok,
                            "attempts": ack.attempts,
                            "status": ack.status,
                            "missing": bool(ack.detail.get("missing", False)),
                            "error": ack.detail.get("error"),
                        }
                        for ack in outcome.acks
                    ],
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
        clock: Callable[[], float] | None = None,
        sleep: Callable[[float], None] | None = None,
    ):
        self.plan = plan
        try:
            self.ordered_names = order(plan)
        except PlanError:
            self.ordered_names = list(plan.names)
        if providers is None:
            self.providers = build_providers(plan, session=session)
        else:
            self.providers = _coerce_providers(plan, providers)
        self.journal_path = journal_path
        self.session = session
        self.clock = clock or time.time
        self.sleep = sleep or time.sleep
        self.outcomes: dict[str, KeyOutcome] = {}
        self.material: dict[str, dict[str, Any]] = {}
        for spec in self.ordered_specs():
            self._outcome(spec)

    def ordered_specs(self) -> list[KeySpec]:
        return [self.plan.by_name(name) for name in self.ordered_names]

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
        self._write(
            spec,
            "create",
            PHASE_CREATED,
            {
                "intent": True,
                "old_key_ref": outcome.old_key_id,
                "slot": spec.slot,
            },
        )
        try:
            material = provider.create(spec.as_request())
        except ProviderError:
            raise
        except Exception as exc:
            raise ProviderError(str(exc)) from exc
        if not isinstance(material, dict) or not material.get("id"):
            raise ProviderError(
                f"{spec.service}: create for {spec.name!r} returned no key id",
                body=material,
            )

        self.material[spec.name] = material
        outcome.new_key_id = material.get("id")
        outcome.created_at = self.clock()
        outcome.phase = PHASE_CREATED
        outcome.create_confirmed = True
        self._write(
            spec,
            "create",
            PHASE_CREATED,
            {
                "ok": True,
                "new_key_ref": material.get("id"),
                "old_key_ref": outcome.old_key_id,
                "created_at": outcome.created_at,
            },
        )
        return material

    def _notify(self, spec: KeySpec, material: dict[str, Any]) -> list[notify.Ack]:
        outcome = self._outcome(spec)
        self._write(
            spec,
            "notify",
            PHASE_NOTIFIED,
            {
                "intent": True,
                "new_key_ref": material.get("id") or outcome.new_key_id,
                "consumers": [consumer.get("name") for consumer in self.plan.consumers],
            },
        )
        dispatched = notify.broadcast(
            self.plan,
            spec,
            material,
            session=self.session,
            sleep=self.sleep,
            clock=self.clock,
        )
        acks = notify.await_acks(
            self.plan,
            spec,
            dispatched,
            session=self.session,
            sleep=self.sleep,
            clock=self.clock,
        )
        outcome.acks = list(acks)
        if not acks.all_ok:
            outcome.notify_complete = False
            self._write(
                spec,
                "notify",
                PHASE_NOTIFIED,
                {
                    "ok": False,
                    "new_key_ref": material.get("id") or outcome.new_key_id,
                    "failed": list(acks.failed),
                    "missing": list(acks.missing),
                },
            )
            raise notify.NotifyError(
                f"acknowledgements incomplete for {spec.name!r}", acks
            )

        outcome.notify_complete = True
        outcome.phase = PHASE_NOTIFIED
        self._write(
            spec,
            "notify",
            PHASE_NOTIFIED,
            {
                "ok": True,
                "new_key_ref": material.get("id") or outcome.new_key_id,
                "acked": list(acks.successful),
            },
        )
        return list(acks)

    def _check_before_revoke(self, spec: KeySpec) -> bool:
        outcome = self._outcome(spec)
        provider = self.providers[spec.service]
        results = probe.verify_both(
            spec,
            self.material.get(spec.name),
            provider=provider,
        )
        outcome.probes = {key: bool(value) for key, value in results.items() if key != "ok"}
        self._write(
            spec,
            "verify",
            outcome.phase if outcome.phase in {PHASE_NOTIFIED, PHASE_DWELL} else PHASE_NOTIFIED,
            {
                "new_key_ref": outcome.new_key_id,
                "old_key_ref": outcome.old_key_id,
                "new_ok": results["new"],
                "old_ok": results["old"],
                "ok": results["ok"],
            },
        )
        return bool(results["ok"])

    def _wait_for_dwell(self, spec: KeySpec) -> None:
        outcome = self._outcome(spec)
        if outcome.created_at is not None:
            remaining = self.plan.dwell - (self.clock() - outcome.created_at)
            if remaining > 0:
                self.sleep(remaining)
        outcome.phase = PHASE_DWELL
        self._write(
            spec,
            "verify",
            PHASE_DWELL,
            {
                "dwell_satisfied": True,
                "new_key_ref": outcome.new_key_id,
                "dwell_seconds": self.plan.dwell,
            },
        )

    def _revoke_old(self, spec: KeySpec) -> None:
        outcome = self._outcome(spec)
        if not outcome.old_key_id:
            outcome.phase = PHASE_REVOKED
            outcome.revoke_complete = True
            self._write(spec, "revoke", PHASE_REVOKED, {"ok": True, "old_key_ref": None})
            return

        provider = self.providers[spec.service]
        created = outcome.created_at if outcome.created_at is not None else self.clock()
        keep_until = _iso(datetime.fromtimestamp(created + self.plan.dwell, tz=timezone.utc))

        self._write(
            spec,
            "revoke",
            PHASE_REVOKED,
            {
                "intent": True,
                "target_phase": PHASE_REVOKED,
                "old_key_ref": outcome.old_key_id,
                "new_key_ref": outcome.new_key_id,
                "keep_until": keep_until,
            },
        )
        try:
            provider.revoke(spec.as_request(), outcome.old_key_id, keep_until=keep_until)
        except Exception as exc:
            outcome.note = f"revoke_failed: {exc}"
            outcome.needs_manual = True
            outcome.revoke_complete = False
            self._write(
                spec,
                "revoke",
                PHASE_REVOKED,
                {
                    "ok": False,
                    "old_key_ref": outcome.old_key_id,
                    "keep_until": keep_until,
                    "error": str(exc),
                },
            )
            if isinstance(exc, ProviderError):
                raise
            raise ProviderError(str(exc)) from exc

        outcome.note = ""
        outcome.phase = PHASE_REVOKED
        outcome.revoke_complete = True
        self._write(
            spec,
            "revoke",
            PHASE_REVOKED,
            {
                "ok": True,
                "old_key_ref": outcome.old_key_id,
                "keep_until": keep_until,
            },
        )


def advance(rotator: Rotator, spec: KeySpec) -> KeyOutcome:
    """Move one key forward: create, notify, hold the swap window, revoke."""
    outcome = rotator._outcome(spec)

    if outcome.phase == PHASE_REVOKED and outcome.revoke_complete:
        return outcome
    if outcome.phase == PHASE_ROLLED_BACK:
        return outcome

    material = rotator.material.get(spec.name)
    if not outcome.new_key_id or material is None:
        material = rotator._create(spec)
    elif outcome.phase == PHASE_PENDING:
        outcome.phase = PHASE_CREATED

    if not outcome.notify_complete:
        rotator._notify(spec, material)

    if not rotator._check_before_revoke(spec):
        raise VerificationError(f"pre-revocation verification failed for {spec.name!r}")
    rotator._wait_for_dwell(spec)
    rotator._revoke_old(spec)
    return outcome


def rollback(rotator: Rotator) -> dict[str, list[str]]:
    """Recover the new keys this batch created where the old key still serves."""

    revoked: list[str] = []
    manual: list[str] = []

    for spec in reversed(rotator.ordered_specs()):
        outcome = rotator.outcomes.get(spec.name)
        if outcome is None:
            continue
        if outcome.phase == PHASE_ROLLED_BACK and outcome.rollback_complete:
            continue

        if outcome.phase == PHASE_REVOKED or outcome.revoke_complete:
            outcome.needs_manual = True
            if not outcome.note:
                outcome.note = "manual_intervention_required: old key already retired"
            manual.append(spec.name)
            rotator._write(
                spec,
                "rollback",
                PHASE_REVOKED,
                {
                    "manual_intervention": True,
                    "reason": "old_key_already_revoked",
                    "new_key_ref": outcome.new_key_id,
                    "old_key_ref": outcome.old_key_id,
                },
            )
            continue

        if outcome.revoke_failed or outcome.needs_manual:
            outcome.needs_manual = True
            manual.append(spec.name)
            rotator._write(
                spec,
                "rollback",
                outcome.phase,
                {
                    "manual_intervention": True,
                    "reason": "revoke_failed_or_uncertain",
                    "new_key_ref": outcome.new_key_id,
                    "old_key_ref": outcome.old_key_id,
                },
            )
            continue

        if not outcome.new_key_id:
            outcome.phase = PHASE_ROLLED_BACK
            outcome.rollback_complete = True
            continue

        provider = rotator.providers[spec.service]
        rotator._write(
            spec,
            "rollback",
            PHASE_ROLLED_BACK,
            {
                "intent": True,
                "target_phase": PHASE_ROLLED_BACK,
                "new_key_ref": outcome.new_key_id,
            },
        )
        try:
            provider.revoke(spec.as_request(), outcome.new_key_id)
        except Exception as exc:
            outcome.note = f"rollback_failed: {exc}"
            outcome.needs_manual = True
            outcome.rollback_complete = False
            manual.append(spec.name)
            rotator._write(
                spec,
                "rollback",
                PHASE_ROLLED_BACK,
                {"ok": False, "new_key_ref": outcome.new_key_id, "error": str(exc)},
            )
            continue

        outcome.phase = PHASE_ROLLED_BACK
        outcome.rollback_complete = True
        outcome.rollback_requested = False
        outcome.needs_manual = False
        revoked.append(spec.name)
        rotator._write(
            spec,
            "rollback",
            PHASE_ROLLED_BACK,
            {"ok": True, "new_key_ref": outcome.new_key_id},
        )

    return {"revoked": revoked, "manual_intervention": manual}


def _result_from_rotator(
    rotator: Rotator,
    *,
    batch: str,
    resumed: bool,
    diagnostics: list[str] | None = None,
) -> BatchResult:
    manual = [name for name, outcome in rotator.outcomes.items() if outcome.needs_manual]
    rolled_back = [
        name
        for name, outcome in rotator.outcomes.items()
        if outcome.phase == PHASE_ROLLED_BACK
    ]
    ok = all(
        outcome.phase == PHASE_REVOKED and not outcome.needs_manual
        for outcome in rotator.outcomes.values()
    )
    return BatchResult(
        batch=batch,
        ok=ok,
        outcomes=dict(rotator.outcomes),
        resumed=resumed,
        diagnostics=list(diagnostics or []),
        manual_intervention=manual,
        rolled_back=rolled_back,
    )


def _rejected_result(plan: Plan, exc: PlanError) -> BatchResult:
    code = getattr(exc, "kind", None) or "invalid_plan"
    return BatchResult(
        batch=plan.batch,
        ok=False,
        error=str(exc),
        error_code=code,
        error_kind=code,
    )


def execute(
    plan_or_path,
    *,
    journal_path: str | None = None,
    providers=None,
    session=None,
    clock: Callable[[], float] | None = None,
    sleep: Callable[[float], None] | None = None,
) -> BatchResult:
    """Run one whole batch and return what happened to each key."""
    if isinstance(plan_or_path, Plan):
        plan = plan_or_path
    else:
        plan = load_plan(plan_or_path)

    try:
        ordered_names = order(plan)
    except PlanError as exc:
        return _rejected_result(plan, exc)

    rotator = Rotator(
        plan,
        providers=providers,
        journal_path=journal_path,
        session=session,
        clock=clock,
        sleep=sleep,
    )
    rotator.ordered_names = ordered_names

    result = BatchResult(batch=plan.batch, ok=False)
    try:
        for name in ordered_names:
            advance(rotator, plan.by_name(name))
    except (ProviderError, PlanError, notify.NotifyError) as exc:
        result.error = str(exc)
        result.error_code = getattr(exc, "kind", None) or "execution_failed"
        result.error_kind = result.error_code
        report = rollback(rotator)
        result.outcomes = dict(rotator.outcomes)
        result.manual_intervention = report["manual_intervention"]
        result.rolled_back = report["revoked"]
        return result

    return _result_from_rotator(rotator, batch=plan.batch, resumed=False)


def _record_detail(record: dict[str, Any]) -> dict[str, Any]:
    detail = record.get("detail")
    return detail if isinstance(detail, dict) else {}


def _coerce_epoch(value: Any) -> float | None:
    if value is None:
        return None
    try:
        return float(value)
    except (TypeError, ValueError):
        pass
    if isinstance(value, str):
        try:
            parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
        except ValueError:
            return None
        if parsed.tzinfo is None:
            parsed = parsed.replace(tzinfo=timezone.utc)
        return parsed.timestamp()
    return None


def _lookup_material_in_provider(
    provider: Any, spec: KeySpec, key_id: str | None
) -> dict[str, Any] | None:
    """Best-effort recovery of secret material from in-memory test providers."""

    created = getattr(provider, "created", None)
    candidates: list[dict[str, Any]] = []
    if isinstance(created, list):
        candidates.extend(item for item in created if isinstance(item, dict))

    store = getattr(provider, "materials", None) or getattr(provider, "keys", None)
    if isinstance(store, dict):
        value = store.get(key_id)
        if isinstance(value, dict):
            candidates.append(value)
        candidates.extend(item for item in store.values() if isinstance(item, dict))
    elif isinstance(store, list):
        candidates.extend(item for item in store if isinstance(item, dict))

    if key_id:
        for item in candidates:
            if item.get("id") == key_id or item.get("key_id") == key_id:
                return dict(item)
        return None

    matching = [
        item
        for item in candidates
        if item.get("slot") in (None, spec.slot)
        and item.get("service") in (None, spec.service)
    ]
    if matching:
        return dict(matching[-1])
    return None


def _hydrate_resume(rotator: Rotator, state: journal.ResumeState) -> None:
    for spec in rotator.ordered_specs():
        name = spec.name
        outcome = rotator._outcome(spec)
        phase = state.phases.get(name, PHASE_PENDING)
        outcome.phase = phase

        key_records = [record for record in state.records if record.get("key") == name]
        new_id = None
        created_at = None
        secret = None
        latest_notify: dict[str, Any] | None = None
        latest_revoke: dict[str, Any] | None = None
        latest_rollback: dict[str, Any] | None = None

        for record in key_records:
            detail = _record_detail(record)
            candidate_id = detail.get("new_key_ref") or detail.get("new_key_id") or detail.get("id")
            if candidate_id:
                new_id = candidate_id
            if detail.get("created_at") is not None:
                created_at = _coerce_epoch(detail["created_at"])
            if detail.get("new_secret") or detail.get("secret"):
                secret = detail.get("new_secret") or detail.get("secret")
            if record.get("action") == "notify":
                latest_notify = record
            if record.get("action") == "revoke":
                latest_revoke = record
            if record.get("action") == "rollback":
                latest_rollback = record

        if new_id:
            outcome.new_key_id = new_id
        if created_at is not None:
            outcome.created_at = created_at
        elif key_records:
            recorded_at = _coerce_epoch(key_records[0].get("ts"))
            outcome.created_at = (
                recorded_at if recorded_at is not None else rotator.clock()
            )

        if phase in {PHASE_NOTIFIED, PHASE_DWELL, PHASE_REVOKED}:
            outcome.notify_complete = True
        if latest_notify is not None:
            detail = _record_detail(latest_notify)
            if detail.get("ok") is False:
                outcome.notify_complete = False
            elif detail.get("intent") is True and detail.get("ok") is not True:
                outcome.notify_complete = False
            elif detail.get("ok") is True or not detail.get("intent"):
                outcome.notify_complete = True

        outcome.create_confirmed = bool(new_id)
        outcome.revoke_complete = phase == PHASE_REVOKED and name in state.done
        outcome.rollback_complete = phase == PHASE_ROLLED_BACK and name in state.done
        if latest_revoke is not None:
            detail = _record_detail(latest_revoke)
            if detail.get("ok") is False or (
                detail.get("intent") is True and detail.get("ok") is not True
            ):
                outcome.revoke_complete = False
        if latest_rollback is not None:
            detail = _record_detail(latest_rollback)
            if detail.get("ok") is False or (
                detail.get("intent") is True and detail.get("ok") is not True
            ):
                outcome.rollback_complete = False
            outcome.rollback_requested = (
                detail.get("intent") is True and detail.get("ok") is not True
            ) or detail.get("ok") is False

        provider = rotator.providers[spec.service]
        stored = _lookup_material_in_provider(provider, spec, outcome.new_key_id)
        if stored is not None:
            outcome.new_key_id = stored.get("id", outcome.new_key_id)
            rotator.material[name] = stored
        elif outcome.new_key_id:
            material = {"id": outcome.new_key_id}
            if secret:
                material["secret"] = secret
            rotator.material[name] = material
        else:
            stored = _lookup_material_in_provider(provider, spec, None)
            if stored is not None and stored.get("id"):
                outcome.new_key_id = stored["id"]
                rotator.material[name] = stored


def _reconcile_minted_keys(rotator: Rotator) -> None:
    missing_states = {"missing", "absent", "not_found", "does_not_exist", "deleted"}
    for spec in rotator.ordered_specs():
        outcome = rotator._outcome(spec)
        if outcome.phase not in {
            PHASE_CREATED,
            PHASE_NOTIFIED,
            PHASE_DWELL,
        }:
            continue

        provider = rotator.providers[spec.service]
        if not outcome.new_key_id:
            stored = _lookup_material_in_provider(provider, spec, None)
            if stored and stored.get("id"):
                outcome.new_key_id = stored["id"]
                rotator.material[spec.name] = stored

        if not outcome.new_key_id:
            continue
        try:
            description = provider.describe(spec.as_request(), outcome.new_key_id)
        except ProviderError as exc:
            if getattr(exc, "status", None) == 404:
                description = {"state": "missing"}
            else:
                raise
        except Exception as exc:
            raise ProviderError(str(exc)) from exc

        state_value = str(
            (description or {}).get("state")
            or (description or {}).get("status")
            or "active"
        ).lower()
        inactive_states = {"revoked", "inactive", "disabled", "deactivated"}
        if state_value in missing_states or state_value in inactive_states:
            outcome.new_key_id = None
            outcome.create_confirmed = False
            outcome.phase = PHASE_PENDING
            rotator.material.pop(spec.name, None)
        else:
            material = rotator.material.get(spec.name) or {"id": outcome.new_key_id}
            if isinstance(description, dict) and description.get("secret"):
                material["secret"] = description["secret"]
            rotator.material[spec.name] = material


def resume_batch(
    plan: Plan,
    journal_path: str,
    *,
    providers=None,
    session=None,
    clock: Callable[[], float] | None = None,
    sleep: Callable[[float], None] | None = None,
) -> BatchResult:
    """Continue a batch that already has journal records on disk."""
    try:
        ordered_names = order(plan)
    except PlanError as exc:
        return _rejected_result(plan, exc)

    state = journal.resume(journal_path)
    rotator = Rotator(
        plan,
        providers=providers,
        journal_path=journal_path,
        session=session,
        clock=clock,
        sleep=sleep,
    )
    rotator.ordered_names = ordered_names
    _hydrate_resume(rotator, state)
    _reconcile_minted_keys(rotator)

    if any(
        outcome.rollback_requested
        or (outcome.phase == PHASE_ROLLED_BACK and not outcome.rollback_complete)
        for outcome in rotator.outcomes.values()
    ):
        rollback(rotator)

    try:
        for name in ordered_names:
            spec = plan.by_name(name)
            outcome = rotator.outcomes[name]
            if outcome.phase == PHASE_REVOKED and outcome.revoke_complete:
                continue
            if outcome.phase == PHASE_ROLLED_BACK:
                continue
            if outcome.needs_manual:
                continue
            advance(rotator, spec)
    except (ProviderError, PlanError, notify.NotifyError) as exc:
        result = BatchResult(
            batch=plan.batch,
            ok=False,
            error=str(exc),
            error_code=getattr(exc, "kind", None) or "execution_failed",
            error_kind=getattr(exc, "kind", None) or "execution_failed",
            resumed=True,
            diagnostics=list(state.diagnostics),
        )
        report = rollback(rotator)
        result.outcomes = dict(rotator.outcomes)
        result.manual_intervention = report["manual_intervention"]
        result.rolled_back = report["revoked"]
        return result

    result = _result_from_rotator(
        rotator,
        batch=plan.batch,
        resumed=True,
        diagnostics=state.diagnostics,
    )
    return result


# Documentation-facing shorthand.
resume = resume_batch
