# hlsmith

把一段**样本流**（每个样本带解码时间戳与随机访问标记）打包成可交付给 CDN 的
**HLS（fMP4）** 目录：一个初始化段 `init.mp4`、若干媒体段 `seg-NNNNN.m4s`、
一个媒体播放列表 `index.m3u8`。

本项目是纯标准库实现（Python 3.10+），不依赖 ffmpeg。素材以 JSON Lines 描述，
样本负载由 `hlsmith/payload.py` 按 `seq` 确定性生成，所以任何人拿到源文件都能重算出
"应该是什么字节"，便于对拍。

## 快速开始

```bash
python -m hlsmith.cli --source samples/vod-20s.jsonl --out out
python -m hlsmith.cli --source samples/vod-20s.jsonl --target 4 --report   # 只看切段表
python -m hlsmith.cli --make-source /tmp/gen.jsonl --seconds 30            # 生成合成源
```

输出：

```
out/
  init.mp4        ftyp + moov
  seg-00000.m4s   moof + mdat
  ...
  index.m3u8
```

## 文档

| 文档 | 内容 |
|---|---|
| [docs/packaging-spec.md](docs/packaging-spec.md) | **出契约**：时间线、切段规则、init/媒体段结构、播放列表要求。实现以这份为准 |
| [docs/incident-reports.md](docs/incident-reports.md) | 线上反馈与问题单 |
| [docs/player-compat.md](docs/player-compat.md) | 已验证播放器、它们的报错原文与已知限制 |

## 目录

```
hlsmith/
  boxes.py      ISO BMFF 盒子的拼装与遍历（只做字节层）
  source.py     源样本流的解析 / 写回 / 合成
  payload.py    样本负载的确定性生成
  segmenter.py  切段决策（切在哪个样本之前）
  init.py       初始化段 ftyp + moov
  media.py      媒体段 moof + mdat
  playlist.py   媒体播放列表
  packager.py   串起来落盘
  cli.py        命令行
docs/           规范、问题单、兼容性记录
examples/       手工排查用的小工具（dump.py）
samples/        两份合成素材
tests/          基线测试
```

## 接口约定

`hlsmith/__init__.py` 导出的 `Source` / `Sample` / `Segment` / `load_source` /
`parse_source` / `segment` 是被下游服务直接引用的稳定接口，参数与返回类型不要变。
CLI 参数同样视为对外契约。

## 开发

```bash
python -m pytest -q
```

## 版本

见 [CHANGELOG.md](CHANGELOG.md)。
