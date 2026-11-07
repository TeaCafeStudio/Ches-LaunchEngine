# 未来 HTTP API 的 JSON 约定

**Planned / Not Implemented（规划中／尚未实现）**。此页只是文档提纲，不定义可调用协议。

实现 HTTP API 时，文档必须明确 Content-Type、请求/响应 Schema、HTTP 状态码、认证与授权、脱敏规则和稳定错误码。目前没有 HTTP 响应封装格式。

正式 CLI 已单独实现 `--json`，其响应为 `{ "success": boolean, "data": value|null, "error": object|null }`。不要假设 HTTP API 也已使用此格式。
