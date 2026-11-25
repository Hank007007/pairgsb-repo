"""Reading submitted policy text into a node tree."""

from collections.abc import Mapping

import yaml

from .anchors import expand


def load_document(text):
    """Read ``text`` and return the document as plain containers.

    The result uses ``MarkedMapping`` / ``MarkedSequence`` for containers so
    that callers can report where a value came from, and carries no aliases:
    every reference has already been turned into its own copy.
    """
    documents = list(yaml.compose_all(text))
    if not documents:
        raise ValueError("document is empty")
    node = documents[0]
    document = expand(node)
    _check_duplicates(document)
    return document


def _check_duplicates(document):
    """Reject a mapping that names the same key twice."""
    if not isinstance(document, Mapping):
        return
    duplicates = getattr(document, "duplicate_keys", ())
    if duplicates:
        key, line = duplicates[0]
        raise ValueError("duplicate key %r on line %s" % (key, line))
