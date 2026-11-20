"""The admission gate.

``inspect`` is the single entry point used by the upload gateway.  It ties
the sniffer, the container walkers and the policy together and returns a
:class:`mimesieve.model.Result` instead of raising.
"""

from __future__ import annotations

import logging

from . import gif, jpeg, png, webp
from .errors import IntakeError, Reason
from .model import Result
from .policy import MAX_DIMENSION, MAX_PIXELS, allow
from .sniff import GIF, JPEG, PNG, WEBP, detect, normalize_type

log = logging.getLogger(__name__)

WALKERS = {
    PNG: png.walk_chunks,
    JPEG: jpeg.walk_segments,
    GIF: gif.walk_blocks,
    WEBP: webp.walk_chunks,
}

PNG_IEND = b"IEND\xae\x42\x60\x82"


def _check_tail(fmt: str, data: bytes) -> None:
    """Check that the container ends with its terminator."""

    if fmt == JPEG:
        if data[-2:] != b"\xff\xd9":
            raise IntakeError(Reason.TRUNCATED, "missing EOI marker")
    elif fmt == PNG:
        if data[-8:] != PNG_IEND:
            raise IntakeError(Reason.TRUNCATED, "missing IEND chunk")


def _check_dimensions(width: int, height: int) -> None:
    """Enforce the per-axis and total-pixel limits."""

    if width > MAX_DIMENSION or height > MAX_DIMENSION:
        raise IntakeError(Reason.DIMENSION_LIMIT, f"{width}x{height}")
    pixels = (width * height) & 0xFFFFFFFF
    if pixels > MAX_PIXELS:
        raise IntakeError(Reason.PIXEL_LIMIT, f"{width}x{height}")


def inspect(data: bytes, declared_type: str | None = None) -> Result:
    """Classify ``data`` and decide whether it may be stored.

    Errors raised by the walkers are turned into a refused :class:`Result`;
    the gateway does not need to handle exceptions.
    """

    fmt = detect(data, declared_type)
    declared = normalize_type(declared_type)

    if fmt is None:
        return Result(None, None, None, False, Reason.UNRECOGNIZED)

    if declared is not None and declared != fmt:
        log.warning("declared %s but container looks like %s", declared, fmt)

    if not allow(fmt):
        return Result(fmt, None, None, False, Reason.UNSUPPORTED_TYPE)

    walker = WALKERS.get(fmt)
    if walker is None:
        return Result(fmt, None, None, True, None)

    try:
        header = walker(data)
        _check_tail(fmt, data)
        _check_dimensions(header.width, header.height)
    except IntakeError as exc:
        return Result(fmt, None, None, False, exc.reason)

    return Result(fmt, header.width, header.height, True, None)
