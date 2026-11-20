"""The admission gate.

``inspect`` is the single entry point used by the upload gateway.  It ties
the sniffer, the container walkers and the policy together and returns a
:class:`mimesieve.model.Result` instead of raising.
"""

from __future__ import annotations

from . import gif, jpeg, png, webp
from .errors import IntakeError, Reason
from .model import Result
from .policy import MAX_DIMENSION, MAX_PIXELS, allow
from .sniff import GIF, JPEG, PNG, WEBP, detect, normalize_type

WALKERS = {
    PNG: png.walk_chunks,
    JPEG: jpeg.walk_segments,
    GIF: gif.walk_blocks,
    WEBP: webp.walk_chunks,
}


def _check_dimensions(width: int, height: int) -> None:
    """Enforce the per-axis and total-pixel limits."""

    if width > MAX_DIMENSION or height > MAX_DIMENSION:
        raise IntakeError(Reason.DIMENSION_LIMIT, f"{width}x{height}")
    if width * height > MAX_PIXELS:
        raise IntakeError(Reason.PIXEL_LIMIT, f"{width}x{height}")


def inspect(data: bytes, declared_type: str | None = None) -> Result:
    """Classify ``data`` and decide whether it may be stored.

    Errors raised by the walkers are turned into a refused :class:`Result`;
    the gateway does not need to handle exceptions.
    """

    declared = normalize_type(declared_type)

    if declared is not None and not allow(declared):
        return Result(None, None, None, False, Reason.UNSUPPORTED_TYPE)

    fmt = detect(data)

    if fmt is None:
        return Result(None, None, None, False, Reason.UNRECOGNIZED)

    if declared is not None and declared != fmt:
        return Result(fmt, None, None, False, Reason.CONTENT_TYPE_MISMATCH)

    try:
        header = WALKERS[fmt](data)
        if header.end != len(data):
            raise IntakeError(
                Reason.TRAILING_DATA, f"{len(data) - header.end} bytes after terminator"
            )
        _check_dimensions(header.width, header.height)
    except IntakeError as exc:
        return Result(fmt, None, None, False, exc.reason)

    return Result(fmt, header.width, header.height, True, None)
