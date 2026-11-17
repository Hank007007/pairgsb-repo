"""Checks for the union relation defined on documents."""

from __future__ import annotations

from relgraph.resolver import Resolver
from relgraph.store import TupleStore


def test_direct_viewer_is_allowed(store: TupleStore, resolver: Resolver) -> None:
    store.add_tuples(["doc:1#viewer@user:alice"])

    assert resolver.check("user:alice", "viewer", "doc:1").allowed


def test_unrelated_user_is_denied(store: TupleStore, resolver: Resolver) -> None:
    store.add_tuples(["doc:1#viewer@user:alice"])

    assert not resolver.check("user:bob", "viewer", "doc:1").allowed


def test_editor_is_viewer_through_union(
    store: TupleStore, resolver: Resolver
) -> None:
    store.add_tuples(["doc:1#editor@user:alice"])

    assert resolver.check("user:alice", "viewer", "doc:1").allowed


def test_viewer_of_another_document_is_denied(
    store: TupleStore, resolver: Resolver
) -> None:
    store.add_tuples(["doc:2#viewer@user:alice"])

    assert not resolver.check("user:alice", "viewer", "doc:1").allowed
