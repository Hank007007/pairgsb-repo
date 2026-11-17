"""Evaluation of authorization questions against stored relation tuples."""

from __future__ import annotations

from typing import Any, Iterable

from relgraph.cache import decision_cache
from relgraph.decision import Decision
from relgraph.expand import MAX_DEPTH, subject_matches
from relgraph.schema import load_schemas, object_type
from relgraph.store import TupleStore


class Resolver:
    """Answers ``check(subject, relation, object)`` questions."""

    def __init__(
        self,
        store: TupleStore,
        schemas: dict[str, dict[str, Any]] | None = None,
    ) -> None:
        self.store = store
        self.schemas = load_schemas() if schemas is None else schemas

    def check(self, subject: str, relation: str, obj: str) -> Decision:
        """Return the decision for one authorization question."""

        return self._check_cached(self.store.revision, subject, relation, obj)

    @decision_cache
    def _check_cached(
        self, revision: int, subject: str, relation: str, obj: str
    ) -> Decision:
        allowed, reasons = self._eval_relation(subject, relation, obj, 0)
        return Decision(allowed, reasons)

    def _eval_relation(
        self,
        subject: str,
        relation: str,
        obj: str,
        depth: int,
    ) -> tuple[bool, tuple[str, ...]]:
        kind = object_type(obj)
        relations = self.schemas.get(kind or "")
        node = relations.get(relation) if relations else None
        if node is None:
            return False, ("unknown_relation",)
        return self._eval_node(node, subject, relation, obj, depth)

    def _eval_node(
        self,
        node: dict[str, Any],
        subject: str,
        relation: str,
        obj: str,
        depth: int,
    ) -> tuple[bool, tuple[str, ...]]:
        kind = node["kind"]

        if kind == "direct":
            return self._eval_direct(subject, relation, obj, depth)

        if kind == "union":
            reasons: tuple[str, ...] = ("union",)
            for child in node["children"]:
                allowed, child_reasons = self._eval_node(
                    child, subject, relation, obj, depth
                )
                if allowed:
                    return True, ("union",)
                reasons = _merge_reasons(reasons, child_reasons)
            return False, reasons

        if kind == "intersection":
            return self._check_intersection(node, subject, relation, obj, depth)

        if kind == "exclusion":
            return self._check_exclusion(node, subject, relation, obj, depth)

        if kind == "computed_userset":
            allowed, reasons = self._eval_relation(
                subject, node["relation"], obj, depth
            )
            return allowed, ("computed_userset",) + reasons

        if kind == "tuple_to_userset":
            return self._eval_tuple_to_userset(node, subject, obj, depth)

        return False, ("unknown_node",)

    def _eval_direct(
        self, subject: str, relation: str, obj: str, depth: int
    ) -> tuple[bool, tuple[str, ...]]:
        reasons: tuple[str, ...] = ("direct",)
        for stored in self.store.subjects_for(obj, relation):
            allowed, match_reasons = subject_matches(
                subject, stored, self.store, depth
            )
            if allowed:
                return True, ("direct",)
            reasons = _merge_reasons(reasons, match_reasons)
        return False, reasons

    def _check_intersection(
        self,
        node: dict[str, Any],
        subject: str,
        relation: str,
        obj: str,
        depth: int,
    ) -> tuple[bool, tuple[str, ...]]:
        reasons: tuple[str, ...] = ("intersection",)
        outcomes = []
        for child in node["children"]:
            allowed, child_reasons = self._eval_node(
                child, subject, relation, obj, depth
            )
            outcomes.append(allowed)
            reasons = _merge_reasons(reasons, child_reasons)
        return all(outcomes), reasons

    def _check_exclusion(
        self,
        node: dict[str, Any],
        subject: str,
        relation: str,
        obj: str,
        depth: int,
    ) -> tuple[bool, tuple[str, ...]]:
        base_allowed, base_reasons = self._eval_node(
            node["base"], subject, relation, obj, depth
        )
        subtract_allowed, subtract_reasons = self._eval_node(
            node["subtract"], subject, relation, obj, depth
        )
        reasons = _merge_reasons(
            ("exclusion",) + base_reasons, subtract_reasons
        )
        if base_allowed and not subtract_allowed:
            return True, reasons
        return False, reasons

    def _eval_tuple_to_userset(
        self,
        node: dict[str, Any],
        subject: str,
        obj: str,
        depth: int,
    ) -> tuple[bool, tuple[str, ...]]:
        computed = node["computed_userset"]
        reasons: tuple[str, ...] = ("tuple_to_userset",)
        exceeded = False
        for parent in self._parents(obj, node["tupleset"]):
            if depth >= MAX_DEPTH:
                exceeded = True
                continue
            allowed, parent_reasons = self._eval_relation(
                subject, computed, parent, depth + 1
            )
            if allowed:
                return True, ("tuple_to_userset",)
            if "depth_exceeded" in parent_reasons:
                exceeded = True
            reasons = _merge_reasons(reasons, parent_reasons)
        if exceeded:
            reasons = _merge_reasons(reasons, ("depth_exceeded",))
        return False, reasons

    def _parents(self, obj: str, relation: str) -> Iterable[str]:
        for parent in sorted(self.store.subjects_for(obj, relation)):
            yield parent.split("#")[0]


def _merge_reasons(
    existing: tuple[str, ...], extra: tuple[str, ...]
) -> tuple[str, ...]:
    merged = list(existing)
    for reason in extra:
        if reason not in merged:
            merged.append(reason)
    return tuple(merged)
