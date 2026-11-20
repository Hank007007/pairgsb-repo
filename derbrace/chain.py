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
            if child.aki is not None and candidate.ski != child.aki:
                continue
            return candidate
    return None


def _require_ca(cert: Certificate) -> None:
    if cert.basic_constraints is None or not cert.basic_constraints[0]:
        raise ChainError(
            "issuing certificate %r is not a CA" % cert.subject
        )
    if cert.key_usage and "keyCertSign" not in cert.key_usage:
        raise ChainError(
            "issuing certificate %r lacks keyCertSign" % cert.subject
        )


def _check_path_len(chain: List[Certificate], index: int) -> None:
    issuer = chain[index]
    constraint = issuer.basic_constraints[1]
    if constraint is None:
        return
    subordinates = 0
    for subordinate in chain[1 : index]:
        if subordinate.subject != subordinate.issuer:
            subordinates += 1
    if constraint < subordinates:
        raise ChainError(
            "pathLenConstraint %d on %r violated by %d subordinate certificate(s)"
            % (constraint, issuer.subject, subordinates)
        )


def verify_chain(
    leaf: Certificate,
    intermediates: Iterable[Certificate],
    anchors: Iterable[Certificate],
    require_client_auth: bool = True,
) -> ChainResult:
    """Build and structurally validate a chain for *leaf*."""
    anchor_list = list(anchors)
    pool = list(intermediates) + anchor_list
    if require_client_auth and leaf.eku and EKU_CLIENT_AUTH not in leaf.eku:
        raise ChainError("leaf certificate lacks id-kp-clientAuth")
    chain = [leaf]
    current = leaf
    seen = {id(leaf)}
    while True:
        if any(current is anchor for anchor in anchor_list):
            return ChainResult(chain=chain, anchor=current)
        issuer = _find_issuer(current, pool)
        if issuer is None:
            raise ChainError("no issuer found for %r" % current.subject)
        if id(issuer) in seen:
            raise ChainError("certificate loop at %r" % issuer.subject)
        _require_ca(issuer)
        seen.add(id(issuer))
        chain.append(issuer)
        _check_path_len(chain, len(chain) - 1)
        current = issuer
