"""Compiling a loaded document into the policy object."""

from .compile import Policy, Rule, compile_policy
from .errors import PolicyError

__all__ = ["Policy", "PolicyError", "Rule", "compile_policy"]
