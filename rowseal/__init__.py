"""rowseal: row-level security in front of the application database.

The package turns per-table policy declarations into SQL boolean
predicates and splices them into statements before they reach the driver.

Typical use::

    from rowseal import Context, load_policy_set, configure, apply

    policy_set = load_policy_set("policies/base.yaml")
    configure(policy_set)
    statement = apply("select * from orders", Context(role="clerk"))
    driver.execute(statement.sql, statement.params)
"""

from .cache import CompileCache, Compiled, compile_cached
from .compile_ import CompileError, compile_expression, compile_policy, parse_expression
from .params import ParamList
from .policy import (
    COMMANDS,
    Context,
    PolicyError,
    PolicySet,
    ResolvedPolicy,
    Rule,
    TablePolicy,
    load_policy_set,
    loads_policy_set,
    resolve_policy,
)
from .query import Rewriter, Statement, apply, compile_query, configure, reset

__all__ = [
    "COMMANDS",
    "CompileCache",
    "CompileError",
    "Compiled",
    "Context",
    "ParamList",
    "PolicyError",
    "PolicySet",
    "ResolvedPolicy",
    "Rewriter",
    "Rule",
    "Statement",
    "TablePolicy",
    "apply",
    "compile_cached",
    "compile_expression",
    "compile_policy",
    "compile_query",
    "configure",
    "load_policy_set",
    "loads_policy_set",
    "parse_expression",
    "reset",
    "resolve_policy",
]

__version__ = "0.4.0"
