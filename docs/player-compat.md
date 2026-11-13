# 兼容性记录

| 播放器 / 工具 | 版本 | 结论 | 备注 |
|---|---|---|---|
| Chromium（MSE） | 141 | ✅ 可播 | 依赖 `tfdt` 连续；时间线跳变只在 seek 时暴露 |
| Safari（macOS） | 26.2 | ⚠️ 看列表 | 对 `EXT-X-TARGETDURATION` 校验严格，偏小直接拒播 |
| `mediastreamvalidator` | 1.23.0 | ⚠️ 看列表 | 报错原文见 [incident-reports.md](incident-reports.md) HLS-233 |
| ffprobe | 7.1 | ✅ 可读 | 读 `mvhd.duration` 判时长；对 `stbl` 为空是正常的（fMP4） |
| Android MediaPlayer | 14 | ⚠️ 看列表 | 缺 `EXT-X-PLAYLIST-TYPE` / `EXT-X-ENDLIST` 会按直播处理 |
| VLC | 3.0.21 | ✅ 可播 | 对不规范列表容忍度高，**不能**拿它当验收依据 |

## 已知限制

- 只做单码率点播；多码率 master 列表与 `EXT-X-STREAM-INF` 不在本期范围。
- 不写 `sidx`（段索引）。CDN 的预取依赖文件名顺序，不依赖 `sidx`。
- 不写加密相关盒子（`pssh` / `sinf`）；DRM 分支在另一条流水线上。

## 验收工具

仓库不带第三方分析工具。`examples/dump.py` 是本仓库自己的盒子转储器，
可以把 init / 媒体段的盒子树打出来，用来核对上面的问题时够用：

```bash
python examples/dump.py out/init.mp4
python examples/dump.py out/seg-00001.m4s
```
