"""Evaluation of authorization questions against stored relation tuples."""

from __future__ import annotations

from typing import Any, Iterable

from relgraph.cache import decision_cache
from relgraph.decision import Decision
from relgraph.expand import expand_subject
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
        allowed, reasons = self._eval_relation(subject, relation, obj)
        return Decision(allowed, reasons)

    def _eval_relation(
        self, subject: str, relation: str, obj: str
    ) -> tuple[bool, tuple[str, ...]]:
        kind = object_type(obj)
        relations = self.schemas.get(kind or "")
        node = relations.get(relation) if relations else None
        if node is None:
            return False, ("unknown_relation",)
        return self._eval_node(node, subject, relation, obj)

    def _eval_node(
        self,
        node: dict[str, Any],
        subject: str,
        relation: str,
        obj: str,
    ) -> tuple[bool, tuple[str, ...]]:
        kind = node["kind"]

        if kind == "direct":
            return self._eval_direct(subject, relation, obj)

        if kind == "union":
            for child in node["children"]:
                allowed, _ = self._eval_node(child, subject, relation, obj)
                if allowed:
                    return True, ("union",)
            return False, ("union",)

        if kind == "intersection":
            return self._check_intersection(node, subject, relation, obj)

        if kind == "exclusion":
            return self._check_exclusion(node, subject, relation, obj)

        if kind == "computed_userset":
            allowed, reasons = self._eval_relation(
                subject, node["relation"], obj
            )
            return allowed, ("computed_userset",) + reasons

        if kind == "tuple_to_userset":
            return self._eval_tuple_to_userset(node, subject, obj)

        return False, ("unknown_node",)

    def _eval_direct(
        self, subject: str, relation: str, obj: str
    ) -> tuple[bool, tuple[str, ...]]:
        target = subject.split(":")[-1]
        for stored in self.store.subjects_for(obj, relation):
            if target in expand_subject(stored, self.store):
                return True, ("direct",)
        return False, ("direct",)

    def _check_intersection(
        self,
        node: dict[str, Any],
        subject: str,
        relation: str,
        obj: str,
    ) -> tuple[bool, tuple[str, ...]]:
        outcomes = [
            self._eval_node(child, subject, relation, obj)[0]
            for child in node["children"]
        ]
        return any(outcomes), ("intersection",)

    def _check_exclusion(
        self,
        node: dict[str, Any],
        subject: str,
        relation: str,
        obj: str,
    ) -> tuple[bool, tuple[str, ...]]:
        base_allowed, base_reasons = self._eval_node(
            node["base"], subject, relation, obj
        )
        subtract_allowed, subtract_reasons = self._eval_node(
            node["subtract"], subject, relation, obj
        )
        return base_allowed, ("exclusion",) + base_reasons

    def _eval_tuple_to_userset(
        self,
        node: dict[str, Any],
        subject: str,
        obj: str,
    ) -> tuple[bool, tuple[str, ...]]:
        computed = node["computed_userset"]
        for parent in self._parents(obj, node["tupleset"]):
            allowed, _ = self._eval_relation(subject, computed, parent)
            if allowed:
                return True, ("tuple_to_userset",)
        return False, ("tuple_to_userset",)

    def _parents(self, obj: str, relation: str) -> Iterable[str]:
        for parent in sorted(self.store.subjects_for(obj, relation)):
            yield parent.split("#")[0]
