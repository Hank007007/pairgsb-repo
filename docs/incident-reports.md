# 问题单汇总（线上反馈）

> 记录来自 CDN 同事、播放器团队和下游服务。**现象按收到的原文抄录**，不做二次加工。
> 规范在 [packaging-spec.md](packaging-spec.md)，实现以规范为准。
> 复现素材：`samples/clip-offset-vfr.jsonl`（21.7 秒，绝对 dts 从 8100000 起）。

---

## HLS-231 seek 到第 2 段就黑屏（11-06，播放器团队）

用 `samples/clip-offset-vfr.jsonl` 打包，Chromium 上顺序播能播完，但一往后拖就黑屏，控制台：

```
Uncaught (in promise) DOMException: Failed to execute 'appendBuffer' on 'SourceBuffer':
The SourceBuffer is full or the media segment does not fit the current period.
```

把前两段的 `tfdt` 打出来是这样：

```
第 1 段 tfdt = 0
第 2 段 tfdt = 360000（4.0 s）
```

素材的绝对解码时间从 8100000 起（录制第 90 秒），产物里却从 0 开始了。
下游字幕组也报过：同一条素材配好的字幕整体前了 90 秒。

## HLS-232 列表里的文件名整体错一位（11-06，CDN）

磁盘上是 `seg-00000.m4s` … `seg-00005.m4s`，m3u8 里列的却是
`seg-00001.m4s` … `seg-00006.m4s`。播放器照列表拉文件，第 1 项跳过素材开头，
最后一项 `seg-00006.m4s` 404，停在结尾不动。

## HLS-233 Safari 直接不播（11-05，播放器团队）

Safari 打开直接拒绝，`video.error.code = 4`，抓包看是播放列表被拒。用 Apple 的
`mediastreamvalidator` 跑了一遍：

```
Error: #EXT-X-TARGETDURATION is smaller than the maximum #EXTINF duration (4.020000)
```

配置的目标段长是 4 秒，播放列表里 `#EXT-X-TARGETDURATION:4`，但有一段的 `EXTINF`
是 4.02 秒（切点要落在关键帧上，一段收尾必然多出几帧）。切点规则修正后长 GOP 会更明显，
一段可能是 7 秒以上；这个值必须按产物里最大的 `EXTINF` 向上取整。

## HLS-234 总时长显示 0.23 秒（11-05，下游水印服务）

水印服务读 init.mp4 的 `mvhd.duration` 做时长规划，读出来的值小得离谱，
最后一条水印直接压到片尾外面去了。同一份素材在 `ffprobe` 看：

```
Duration: 00:00:00.24, start: 0.000000, bitrate: ...
```

素材实长 21.7 秒、timescale 90000，`mvhd.duration` 读出来是 21700 —— 看着像毫秒，
实际被当成 1/90000 秒的 ticks 去解释了。`mdhd` / `tkhd` 里的 duration 也是同一类数值。

## HLS-235 变帧率素材音画不同步（11-04，播放器团队）

`samples/clip-offset-vfr.jsonl` 混着 3000 和 3600 ticks 的帧（30fps 与 25fps 混剪，
第 1 段 114 个样本里就有 30 个 3600）。播过一段时间声音越来越超前。把某段 `trun`
里每个样本条目的 duration 打出来：

```
sample 0..113 : duration = 3000   (整段同一个值)
```

该段首样本是 3000，后面那些 3600 的帧也被当成 3000 写了。按样本累加出来的段时长
与播放列表里的 `EXTINF` 对不上，音画从这里开始漂。

## HLS-236 尾段只有 1.667 秒（11-04，CDN 运维）

最后一段只剩 1.667 秒（50 个样本），播放器在结尾有一次明显卡顿，HLS 日志：

```
playlist duration 21.7s, loaded 21.00s, stall at 20.98s
```

规范要求尾段小于半个目标段长（本例 2 秒）时并入前一段。这次没有并。

## HLS-237 某些播放器不认点播列表（11-03，兼容性）

部分 Android 播放器把这个 m3u8 当成直播列表处理，进度条不能拖。要求里缺
`#EXT-X-PLAYLIST-TYPE:VOD`，结尾也缺 `#EXT-X-ENDLIST`。

## HLS-238 切点落在非关键帧上（11-02，播放器团队）

抽查同一份素材，从第 2 段起**每一段**的首个样本 `sync = false`（第 2 段首个样本
`seq = 114`），seek 到这些段开头会花屏约 0.3 秒。规范要求每段必须从 sync 样本开始。
