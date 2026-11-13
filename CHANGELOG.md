# Changelog

## Unreleased

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
