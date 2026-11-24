"""Batch description: reading a config file and deriving an execution order.

See ``docs/rotation-plan.md`` for the rules this module implements.
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from typing import Any, Iterable


class PlanError(ValueError):
    """Raised when a batch description cannot be executed as written."""


@dataclass(frozen=True)
class KeySpec:
    """One credential slot inside a batch."""

    name: str
    service: str
    slot: str
    depends_on: tuple[str, ...] = ()
    current: dict[str, Any] | None = None

    def as_request(self) -> dict[str, Any]:
        """The plain mapping handed to provider adapters."""
        return {
            "name": self.name,
            "service": self.service,
            "slot": self.slot,
            "depends_on": list(self.depends_on),
        }


DEFAULT_NOTIFY = {
    "max_attempts": 5,
    "backoff_base": 1.0,
    "backoff_cap": 8.0,
    "ack_timeout": 60.0,
    "poll_interval": 1.0,
}


@dataclass
class Plan:
    """A parsed batch."""

    batch: str
    dwell: float
    services: dict[str, dict[str, Any]]
    keys: list[KeySpec]
    consumers: list[dict[str, Any]] = field(default_factory=list)
    notify: dict[str, Any] = field(default_factory=lambda: dict(DEFAULT_NOTIFY))
    path: str | None = None

    def by_name(self, name: str) -> KeySpec:
        for spec in self.keys:
            if spec.name == name:
                return spec
        raise KeyError(name)

    @property
    def names(self) -> list[str]:
        return [spec.name for spec in self.keys]


def _require(condition: bool, message: str) -> None:
    if not condition:
        raise PlanError(message)


def _as_key_spec(raw: Any, index: int, services: dict[str, Any]) -> KeySpec:
    _require(isinstance(raw, dict), f"keys[{index}] must be an object")
    name = raw.get("name")
    _require(isinstance(name, str) and name.strip() != "",
             f"keys[{index}].name must be a non-empty string")
    service = raw.get("service") or name.split(".", 1)[0]
    _require(isinstance(service, str) and service != "",
             f"keys[{index}].service must be a non-empty string")
    _require(service in services,
             f"key {name!r} refers to undeclared service {service!r}")
    slot = raw.get("slot") or name.split(".", 1)[-1]
    depends_on = raw.get("depends_on") or []
    _require(isinstance(depends_on, list) and all(isinstance(d, str) for d in depends_on),
             f"key {name!r}: depends_on must be a list of names")
    current = raw.get("current")
    if current is not None:
        _require(isinstance(current, dict) and isinstance(current.get("id"), str),
                 f"key {name!r}: current must be an object with an id")
    return KeySpec(
        name=name,
        service=service,
        slot=slot,
        depends_on=tuple(depends_on),
        current=current,
    )


def load_plan(path: str) -> Plan:
    """Read a batch description from ``path``.

    Only the shape of the document is checked here; whether the batch can be
    executed at all is decided by :func:`order` and :func:`detect_cycle`.
    """
    with open(path, "r", encoding="utf-8") as handle:
        raw = json.load(handle)
    _require(isinstance(raw, dict), "plan must be a JSON object")

    batch = raw.get("batch")
    _require(isinstance(batch, str) and batch != "", "plan.batch must be a non-empty string")

    dwell = raw.get("dwell", 0)
    _require(isinstance(dwell, (int, float)) and dwell >= 0, "plan.dwell must be a non-negative number")

    services = raw.get("services")
    _require(isinstance(services, dict) and services, "plan.services must be a non-empty object")

    raw_keys = raw.get("keys")
    _require(isinstance(raw_keys, list) and raw_keys, "plan.keys must be a non-empty array")

    keys = [_as_key_spec(item, i, services) for i, item in enumerate(raw_keys)]

    seen: set[str] = set()
    for spec in keys:
        _require(spec.name not in seen, f"duplicate key name {spec.name!r}")
        seen.add(spec.name)

    consumers = raw.get("consumers") or []
    _require(isinstance(consumers, list), "plan.consumers must be an array")
    for i, consumer in enumerate(consumers):
        _require(isinstance(consumer, dict) and isinstance(consumer.get("name"), str),
                 f"consumers[{i}] must be an object with a name")
        _require(isinstance(consumer.get("endpoint"), str) and consumer["endpoint"],
                 f"consumers[{i}] must have an endpoint")

    notify = dict(DEFAULT_NOTIFY)
    notify.update(raw.get("notify") or {})

    return Plan(
        batch=batch,
        dwell=float(dwell),
        services=dict(services),
        keys=keys,
        consumers=[dict(c) for c in consumers],
        notify=notify,
        path=path,
    )


def _known_dependencies(plan: Plan) -> dict[str, list[str]]:
    names = set(plan.names)
    return {spec.name: [d for d in spec.depends_on if d in names] for spec in plan.keys}


def detect_cycle(plan: Plan) -> list[str] | None:
    """Return the key names forming a dependency cycle, or ``None``."""
    graph = _known_dependencies(plan)
    colour: dict[str, int] = {name: 0 for name in graph}
    stack: list[str] = []

    def visit(node: str) -> list[str] | None:
        colour[node] = 1
        stack.append(node)
        for nxt in graph.get(node, ()):
            if colour.get(nxt, 0) == 1:
                start = stack.index(nxt)
                return stack[start:] + [nxt]
            if colour.get(nxt, 0) == 0:
                found = visit(nxt)
                if found is not None:
                    return found
        stack.pop()
        colour[node] = 2
        return None

    for name in graph:
        if colour[name] == 0:
            found = visit(name)
            if found is not None:
                return found
    return None


def _check_dependencies(plan: Plan) -> None:
    names = set(plan.names)
    problems = []
    for spec in plan.keys:
        for dep in spec.depends_on:
            if dep not in names:
                problems.append(f"key {spec.name!r} depends on unknown key {dep!r}")
            elif dep == spec.name:
                problems.append(f"key {spec.name!r} depends on itself")
    if problems:
        raise PlanError("; ".join(problems))


def order(plan: Plan) -> list[str]:
    """Return the batch's key names in the order they must be processed.

    The rule is the one written down in ``docs/rotation-plan.md`` §2.4: at
    every step take the earliest still-eligible key in declaration order.
    """
    _check_dependencies(plan)
    cycle = detect_cycle(plan)
    if cycle is not None:
        raise PlanError("dependency cycle: " + " -> ".join(cycle))

    remaining: list[str] = list(plan.names)
    placed: list[str] = []
    placed_set: set[str] = set()
    deps = {spec.name: list(spec.depends_on) for spec in plan.keys}

    while remaining:
        picked = None
        for name in remaining:
            if all(dep in placed_set for dep in deps[name]):
                picked = name
                break
        if picked is None:
            raise PlanError("dependency graph cannot be ordered: " + ", ".join(remaining))
        remaining.remove(picked)
        placed.append(picked)
        placed_set.add(picked)
    return placed


def iter_ordered(plan: Plan) -> Iterable[KeySpec]:
    for name in order(plan):
        yield plan.by_name(name)
