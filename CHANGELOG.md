# Changelog

## Unreleased

- 保留源时间线绝对值：媒体段 `tfdt` 取该段首样本在源里的 `dts`，不再归零（HLS-231）。
- 媒体段文件名与播放列表统一从 `seg-00001.m4s` 起编号，修正整体错位（HLS-232）。
- `EXT-X-TARGETDURATION` 按产物实际最大段长向上取整，不再使用配置的目标段长（HLS-233）。
- init 段 `mvhd` / `mdhd` / `tkhd` 的 duration 一律使用 timescale ticks，不再换算毫秒（HLS-234）。
- `trun` 逐样本写入各自的 duration，修复变帧率素材的音画漂移（HLS-235）。
- 短于半个目标段长的尾段并入前一段（HLS-236）。
- 播放列表补齐 `EXT-X-PLAYLIST-TYPE:VOD` 与 `EXT-X-ENDLIST`（HLS-237）。
- 切点只落在 sync 样本上，确保每个媒体段都从关键帧开始（HLS-238）。
- 支持 `--report` 只打印切段表，不落盘。
- `examples/dump.py`：盒子转储小工具，排查打包产物用。

## 0.6.0 — 2026-10-28

- 媒体段改用 `tfhd` 的 default-base-is-moof，去掉了对 `sidx` 的依赖。
- `tfdt` 升到 version 1（64 位 `baseMediaDecodeTime`）。
- 播放列表升到 `EXT-X-VERSION:7`，改用 `EXT-X-MAP` 声明初始化段。

## 0.5.2 — 2026-09-30

- 源文件解析补充 `dts` 单调性与首样本必须为 sync 的校验，报错带行号。
- `payload.sample_payload` 改为按 `seq` 确定性生成，便于对拍。

## 0.5.0 — 2026-09-11

- 新增 `packager.package()`，一条命令产出 init / media / m3u8。
- CLI 增加 `--make-source`，可以生成合成素材。

## 0.4.0 — 2026-08-20

- 初始版本：切段、init 段、媒体段、播放列表四块拆开。
