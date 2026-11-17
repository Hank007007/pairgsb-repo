# 变更记录

## [Unreleased]

### Fixed

- `allow_hosts` 精确项不再按字符串后缀匹配，`*.example.com` 也不再匹配 `example.com` 本身
- 未配置 `allow_hosts` 时，主机名会先解析并检查所有地址；回环、私网、链路本地、未指定地址与 CGNAT 一律拒绝
- 抓取只使用准入校验时解析出的地址建连，避免 DNS rebinding；重定向的每一跳都重新执行主机、地址、端口与 scheme 校验
- `allowed_ports` 在准入阶段真正生效，未写端口时按 http=80、https=443 判断
- 重定向跟随受 `max_redirects` 限制，相对地址按上一跳解析
- 响应体按 `max_bytes` 流式读取和计算摘要，超限标记 `truncated`，不再整包读入内存
- API 对策略拒绝统一返回 403 `policy_denied`，解析失败与上游失败返回 502 `upstream_failed`

## [1.4.0] - 2026-10-18

### Added

- `POST /fetch` 出方向抓取接口，只返回状态码、字节数与正文摘要
- `internal/policy` 准入策略：允许名单、拒绝名单、端口白名单
- `internal/fetcher` 抓取客户端，含响应体大小上限

## [1.3.0] - 2026-09-02

### Changed

- 抓取结果不再回传响应头，只回传状态码与摘要

## [1.2.0] - 2026-07-21

### Added

- 单机部署脚本与 `/healthz` 探活
