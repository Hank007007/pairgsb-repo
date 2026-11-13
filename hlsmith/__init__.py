"""hlsmith —— 把样本流打成 HLS（fMP4）输出。

对外稳定接口（本项目约定不得改变参数与返回类型）：

    Source / load_source / parse_source / format_source
    segment(source, target_seconds) -> list[Segment]
    build_init(...) -> bytes
    build_segment(...) -> bytes
    build_playlist(...) -> str
"""

from __future__ import annotations

__version__ = "0.6.0"

from .segmenter import Segment, segment
from .source import Sample, Source, load_source, parse_source

__all__ = [
    "__version__",
    "Sample",
    "Segment",
    "Source",
    "load_source",
    "parse_source",
    "segment",
]
