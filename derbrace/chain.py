"""Certificate chain construction and structural validation."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Iterable, List, Optional, Sequence

from .errors import ChainError
from .x509 import EKU_CLIENT_AUTH, Certificate


@dataclass
class ChainResult:
    chain: List[Certificate]
    anchor: Optional[Certificate] = None

    @property
    def ok(self) -> bool:
        return bool(self.chain)


def _find_issuer(child: Certificate, pool: Sequence[Certificate]) -> Optional[Certificate]:
    for candidate in pool:
        if candidate is child:
            continue
        if candidate.subject == child.issuer:
            return candidate
    return None


def verify_chain(
    leaf: Certificate,
    intermediates: Iterable[Certificate],
    anchors: Iterable[Certificate],
    require_client_auth: bool = True,
) -> ChainResult:
    """Build and structurally validate a chain for *leaf*."""
    pool = list(intermediates) + list(anchors)
    chain = [leaf]
    current = leaf
    seen = {id(leaf)}
    while True:
        issuer = _find_issuer(current, pool)
        if issuer is None:
            if current.subject == current.issuer:
                return ChainResult(chain=chain, anchor=current)
            raise ChainError("no issuer found for %r" % current.subject)
        if id(issuer) in seen:
            raise ChainError("certificate loop at %r" % issuer.subject)
        seen.add(id(issuer))
        chain.append(issuer)
        current = issuer
        if current in anchors:
            return ChainResult(chain=chain, anchor=current)

    return ChainResult(chain=chain)
