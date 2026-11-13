# Changelog

## Unreleased

- 修复媒体段 `tfdt` 被错误归零的问题，保留源时间线的绝对 `dts`（HLS-231）。
- 修复媒体段落盘文件名与播放列表编号整体错一位的问题（HLS-232）。
- 修复 `EXT-X-TARGETDURATION` 未按实际最大 `EXTINF` 向上取整的问题（HLS-233）。
- 修复 init 中 `mvhd` / `mdhd` / `tkhd` 时长被错误换算成毫秒的问题（HLS-234）。
- 修复 `trun` 用首样本时长覆盖整段、变帧率素材时长漂移的问题（HLS-235）。
- 修复尾段短于半个目标段长时未并入前一段的问题（HLS-236）。
- 补齐点播播放列表的 `EXT-X-PLAYLIST-TYPE:VOD` 与 `EXT-X-ENDLIST`（HLS-237）。
- 修复切点可能落在非 sync 样本上的问题（HLS-238）。
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
