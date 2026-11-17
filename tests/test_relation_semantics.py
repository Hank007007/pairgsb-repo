"""Edge-case semantics from docs/relation-model.md."""

from __future__ import annotations

import pytest

from relgraph.expand import MAX_DEPTH
from relgraph.resolver import Resolver
from relgraph.store import TupleStore


def test_intersection_requires_every_child(
    store: TupleStore, resolver: Resolver
) -> None:
    store.add_tuples(["doc:1#reviewer@user:alice"])

    decision = resolver.check("user:alice", "reviewer", "doc:1")
    assert not decision.allowed

    store.add_tuples(["doc:1#editor@user:alice"])
    assert resolver.check("user:alice", "reviewer", "doc:1").allowed


def test_exclusion_denies_subtracted_subject(
    store: TupleStore, resolver: Resolver
) -> None:
    store.add_tuples(
        [
            "doc:1#viewer@user:bob",
            "doc:1#blocked@user:bob",
        ]
    )

    decision = resolver.check("user:bob", "viewer_not_blocked", "doc:1")
    assert not decision.allowed


def test_exclusion_allows_subject_without_block(
    store: TupleStore, resolver: Resolver
) -> None:
    store.add_tuples(["doc:1#viewer@user:carol"])

    assert resolver.check(
        "user:carol", "viewer_not_blocked", "doc:1"
    ).allowed


def test_userset_subject_expands_group_members(
    store: TupleStore, resolver: Resolver
) -> None:
    store.add_tuples(
        [
            "group:eng#member@user:alice",
            "doc:1#viewer@group:eng#member",
        ]
    )

    assert resolver.check("user:alice", "viewer", "doc:1").allowed
    assert not resolver.check("user:eng#member", "viewer", "doc:1").allowed


def test_userset_membership_is_transitive(
    store: TupleStore, resolver: Resolver
) -> None:
    store.add_tuples(
        [
            "group:all#member@group:eng#member",
            "group:eng#member@user:alice",
            "doc:1#viewer@group:all#member",
        ]
    )

    assert resolver.check("user:alice", "viewer", "doc:1").allowed


def _parent_chain(store: TupleStore, length: int) -> None:
    tuples = [
        f"doc:{level + 1}#parent@doc:{level + 2}"
        for level in range(length)
    ]
    tuples.append(f"doc:{length + 1}#viewer@user:alice")
    store.add_tuples(tuples)


def test_parent_chain_at_depth_bound_is_allowed(
    store: TupleStore, resolver: Resolver
) -> None:
    _parent_chain(store, MAX_DEPTH)

    assert resolver.check("user:alice", "viewer", "doc:1").allowed


def test_parent_chain_past_depth_bound_is_denied(
    store: TupleStore, resolver: Resolver
) -> None:
    _parent_chain(store, MAX_DEPTH + 15)

    decision = resolver.check("user:alice", "viewer", "doc:1")
    assert not decision.allowed
    assert "depth_exceeded" in decision.reasons


def test_parent_cycle_terminates_with_depth_exceeded(
    store: TupleStore, resolver: Resolver
) -> None:
    store.add_tuples(
        [
            "doc:1#parent@doc:2",
            "doc:2#parent@doc:1",
        ]
    )

    decision = resolver.check("user:alice", "viewer", "doc:1")
    assert not decision.allowed
    assert "depth_exceeded" in decision.reasons


def test_membership_cycle_terminates_with_depth_exceeded(
    store: TupleStore, resolver: Resolver
) -> None:
    store.add_tuples(
        [
            "group:a#member@group:b#member",
            "group:b#member@group:a#member",
            "doc:1#viewer@group:a#member",
        ]
    )

    decision = resolver.check("user:alice", "viewer", "doc:1")
    assert not decision.allowed
    assert "depth_exceeded" in decision.reasons


def test_remove_is_visible_to_next_check_in_same_process(
    store: TupleStore, resolver: Resolver
) -> None:
    store.add_tuples(["doc:1#viewer@user:bob"])
    assert resolver.check("user:bob", "viewer", "doc:1").allowed

    store.remove_tuples(["doc:1#viewer@user:bob"])

    assert not resolver.check("user:bob", "viewer", "doc:1").allowed


def test_unknown_relation_is_denied(
    store: TupleStore, resolver: Resolver
) -> None:
    decision = resolver.check("user:alice", "nope", "doc:1")
    assert not decision.allowed


@pytest.mark.parametrize("bad_tuple", ["", "doc:1", "doc:1#viewer", "doc1#viewer@user:alice"])
def test_malformed_tuple_is_rejected(
    store: TupleStore, bad_tuple: str
) -> None:
    with pytest.raises(ValueError):
        store.add_tuples([bad_tuple])
