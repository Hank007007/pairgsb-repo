# hlsmith 打包规范（Packaging Spec v3）

> 这份文档是**对外契约**：CDN 与播放器按它解析产物，下游字幕/水印服务按它的时间线对齐。
> 实现与本文冲突时以本文为准。修改本文需走变更评审，不能在修 bug 时顺手改规范。

约定用词：**必须 / 不得** = 硬性要求；**应** = 默认这么做，偏离需在 CHANGELOG 说明。

---

## 1. 时间基与时间线

1.1 源文件头里的 `timescale` 是**唯一**时间基。输出中所有时间字段（`mvhd.duration`、
`mdhd.duration`、`tkhd.duration`、`tfdt.baseMediaDecodeTime`）一律以它为单位。
不得换算成毫秒或将时间基改为 1000。

1.2 源素材是从更长的录制里截出来的，`dts` 不从 0 开始很常见（例如从录制第 3 分钟起截）。
本工具**保留源时间线的绝对值**，不做重定基：

- 媒体段的 `tfdt.baseMediaDecodeTime` 必须等于该段首样本在**源里的** `dts`；
- 整个输出中**不得**对时线做任何平移或归零。

> 理由：下游字幕、水印、广告插入都按绝对解码时间对齐；把时间线归零会让这些轨道整体错位，
> 而单段顺序播放时又看不出问题 —— 所以这类错误只能靠对拍或跳转播放暴露。

1.3 段时长按样本时长累加计算：
`段时长 = 段末样本.dts + 段末样本.dur − 段首样本.dts`。
不得用"下一段起点 − 本段起点"来算（最后一段没有下一段）。

---

## 2. 切段规则

2.1 目标段长 `target` 以秒给出，换算成 ticks：`round(target × timescale)`。

2.2 **段必须从 sync 样本开始**。首段的首样本天然是 sync（源文件已保证），
除此之外每个段的首样本在源里必须 `sync = true`。

2.3 在满足 2.2 的前提下，切点取**第一个**满足下述条件的 sync 样本：
「该样本的 `dts` − 当前段首样本的 `dts` ≥ 目标 ticks」。
若某个 sync 样本到段首已经超过两倍目标 ticks，说明中间缺 sync，仍按"第一个满足条件的 sync"切，
不得切在非 sync 样本上。

2.4 单个 GOP 长于目标段长是允许的：段长会超过 `target`，播放列表按实际最大段长报
`EXT-X-TARGETDURATION`（见 §5.3），不得为了凑长度把段切开。

2.5 尾段若最后一段的时长 **< 0.5 × target**，必须并入前一段，不得产生过短的尾段。
若整个素材只有一段，则保留该段（即使它短于 0.5 × target）。

2.6 段序号从 1 开始，按输出顺序连续递增；样本区间首尾相接、不重不漏，覆盖源的全部样本。

---

## 3. 媒体段（`seg-NNNNN.m4s`）

3.1 结构必须是 `moof` 紧跟 `mdat`，且一个媒体段只含一个 `mfhd`、一个 `traf`。

3.2 `mfhd.sequence_number` 从 1 开始，与播放列表中的段顺序一一对应，连续递增。

3.3 `tfhd`：`track_ID = 1`；标志位必须包含 **default-base-is-moof（0x020000）**。

3.4 `tfdt`：version 1（64 位 `baseMediaDecodeTime`），取值见 §1.2。

3.5 `trun`：

- flags 必须包含 `data-offset-present`、`sample-duration-present`、`sample-size-present`；
- `sample_count` = 段内样本数；
- 每个样本条目的 duration / size 必须取**该样本自己**的值。源素材是变帧率的
  （同一素材里可能出现 3000 / 3600 ticks 等不同时长），不得用首样本的时长套用到整段；
- `data_offset` = 从 `moof` 起始地址到 `mdat` 负载起始的字节数（即整个 `moof` 的长度）。

3.6 `mdat` 负载：按段内样本顺序，每个样本写入 `payload.sample_payload(seq, size)` 的结果
（该函数是确定性的，任何一方都能重算校验）。

3.7 段内不得包含不属于本段的样本，也不得遗漏样本；`moof` 与 `mdat` 的大小必须与实际一致。

---

## 4. 初始化段（`init.mp4`）

4.1 结构：`ftyp` + `moov(mvhd, trak(tkhd, mdia(mdhd, hdlr, minf))))`；
`ftyp` 的 major brand 为 `iso5`。

4.2 `mvhd.timescale` 与 `mdhd.timescale` 都必须等于源 `timescale`。

4.3 `mvhd.duration`、`mdhd.duration`、`tkhd.duration` 三处都必须等于
**全部样本 duration 之和**（单位见 §1.1），不得换算成毫秒或秒。

4.4 `stbl` 中的 `stts` / `stsc` / `stsz` / `stco` 条目数为 0：样本表由每个媒体段的
`moof` 承载，这是 fMP4 与普通 MP4 的区别。

4.5 `hdlr.handler_type` 为 `vide`。

---

## 5. 播放列表（`index.m3u8`）

依据 RFC 8216。本工具的产物是**点播单码率**，规则如下。

5.1 首行必须是 `#EXTM3U`；`#EXT-X-VERSION` 不得低于 **7**（使用 `EXT-X-MAP` 的最低要求）。

5.2 必须包含 `#EXT-X-PLAYLIST-TYPE:VOD`，且必须在 `#EXT-X-MAP` 之前。

5.3 `#EXT-X-TARGETDURATION` 必须等于
**所有段实际时长的最大值向上取整**（RFC 8216 §4.3.3.1：任一段的 `EXTINF` 都不得大于它）。
不得填配置里的目标段长 —— 单个 GOP 长于目标段长时段长会超出，填配置值会直接被播放器拒绝。

5.4 `#EXT-X-MAP:URI="init.mp4"` 必须出现在第一个 `EXTINF` 之前。

5.5 每段两行：

```
#EXTINF:<该段时长，保留 6 位小数>,
<该段的文件名>
```

5.6 文件名：媒体段从 `seg-00001.m4s` 开始编号（1 起），与落盘文件名**逐字一致**。
第 N 段（从 1 数）必须叫 `seg-%05d.m4s`，序号不得错位。

5.7 点播列表必须以 `#EXT-X-ENDLIST` 结尾，其后不得再有内容。

5.8 `#EXT-X-MEDIA-SEQUENCE` 在点播列表里应为 0。

---

## 6. 对外接口与行为

6.1 `hlsmith/__init__.py` 导出的 `Source` / `Sample` / `Segment` / `load_source` /
`parse_source` / `segment`，以及 `packager.package()`、`media.build_segment()`、
`playlist.build_playlist()`、`init.build_init()` 的**参数与返回类型**不得改变，
也不得新增必填参数。

6.2 CLI 参数（`--source` / `--out` / `--target` / `--report` / `--make-source` /
`--seconds` / `--gop`）保持不变。

6.3 依赖：只允许 Python 标准库。不得引入 ffmpeg、m3u8、av 等第三方包。

6.4 产物目录里只应有 `init.mp4`、`seg-NNNNN.m4s`、`index.m3u8` 三类文件；
调试产物（日志、临时文件）不得留在输出目录。

---

## 7. 自检口径（交付前必须能跑通）

7.1 用同一个源跑两次输出，除时间戳字段外产物字节一致。

7.2 从产物反解：每个段的 `tfdt` 等于该段首个样本在源里的 `dts`；
每个段的 `trun` 条目数等于段内样本数；各样本 duration 之和等于 `EXTINF × timescale`。

7.3 播放列表里列出的每个文件都真实存在，且大小与 `moof + mdat` 一致。

7.4 一个源至少要有一份"从非 0 `dts` 开始 + 变帧率 + 含长 GOP"的素材用于验收，
见 `samples/`。
