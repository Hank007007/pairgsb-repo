"""Alias expansion for composed documents, and the limits expansion runs under.

A composed document is a tree of ``yaml.nodes`` objects.  An alias does not
show up as a separate node: every reference to ``*name`` points at the very
node the anchor was declared on.  :func:`expand` walks that tree and turns it
into ordinary Python containers.
"""

import yaml
from yaml.nodes import MappingNode, ScalarNode, SequenceNode

from ..policy.errors import PolicyError
from ..values import MarkedMapping, MarkedSequence

MAX_NODES = 10000
MAX_ALIAS_DEPTH = 4

MERGE_KEY = "<<"


class Budget:
    """Running totals for one expansion.

    ``charge`` adds to the expanded-node total and refuses to go past
    ``max_nodes``.  ``enter_alias`` / ``leave_alias`` bracket the materialisation
    of one alias reference and refuse to nest past ``max_alias_depth``.
    """

    def __init__(self, max_nodes=MAX_NODES, max_alias_depth=MAX_ALIAS_DEPTH):
        self.max_nodes = max_nodes
        self.max_alias_depth = max_alias_depth
        self.nodes = 0
        self.alias_depth = 0

    def charge(self, count=1):
        self.nodes += count
        if self.nodes > self.max_nodes:
            raise PolicyError(
                "node_limit",
                "expanded document is larger than %d nodes" % self.max_nodes,
            )

    def enter_alias(self):
        self.alias_depth += 1
        if self.alias_depth > self.max_alias_depth:
            raise PolicyError(
                "alias_depth",
                "aliases nest deeper than %d levels" % self.max_alias_depth,
            )

    def leave_alias(self):
        if self.alias_depth > 0:
            self.alias_depth -= 1


def expand(node, budget=None):
    """Turn a composed node tree into plain containers.

    ``node`` is the root produced by ``yaml.compose``.  Mappings become
    ``MarkedMapping``, sequences become ``MarkedSequence`` and scalars become
    the value their tag denotes.  A node the walk meets a second time stands
    for an alias reference and is materialised from its own text.
    """
    if budget is None:
        budget = Budget()
    return _expand_node(node, budget, set())


def _expand_node(node, budget, seen):
    budget = Budget(budget.max_nodes, budget.max_alias_depth)
    budget.charge()
    marker = id(node)
    if marker in seen:
        return _materialize(node, budget)
    seen.add(marker)
    if isinstance(node, ScalarNode):
        return _scalar_value(node)
    if isinstance(node, SequenceNode):
        result = MarkedSequence(line=_line(node))
        for item in node.value:
            result.append(_expand_node(item, budget, seen))
        return result
    if isinstance(node, MappingNode):
        result = MarkedMapping(line=_line(node))
        for key_node, value_node in node.value:
            key = _mapping_key(key_node)
            if key in result:
                result.duplicate_keys.append((key, _line(key_node)))
            result[key] = _expand_node(value_node, budget, seen)
        return result
    raise ValueError("unsupported node: %r" % (node,))


def _materialize(node, budget):
    """Build the value an alias stands for from the text of its target."""
    budget.enter_alias()
    try:
        text = yaml.serialize(node)
        return yaml.full_load(text)
    finally:
        budget.leave_alias()


def _scalar_value(node):
    loader = yaml.SafeLoader("")
    try:
        return loader.construct_object(node, deep=True)
    except yaml.YAMLError:
        return node.value
    finally:
        loader.dispose()


def _mapping_key(node):
    if not isinstance(node, ScalarNode):
        raise ValueError("a mapping key must be a scalar")
    if node.tag == "tag:yaml.org,2002:merge":
        return MERGE_KEY
    return _scalar_value(node)


def _line(node):
    mark = getattr(node, "start_mark", None)
    if mark is None:
        return None
    return mark.line + 1
