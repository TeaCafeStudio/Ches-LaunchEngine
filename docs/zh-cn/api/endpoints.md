# 规划中的 HTTP 端点

**Planned / Not Implemented（规划中／尚未实现）**。以下路由均未注册，当前不可调用：

- `GET /api/v1/accounts`
- `POST /api/v1/accounts`
- `GET /api/v1/accounts/{id}`
- `POST /api/v1/accounts/{id}/validate`
- `POST /api/v1/accounts/{id}/refresh`
- `POST /api/v1/accounts/{id}/logout`
- `DELETE /api/v1/accounts/{id}`
- `POST /api/v1/auth/microsoft/start`
- `GET /api/v1/auth/microsoft/callback`

实现前需定义调用方认证/授权、OAuth 回调策略、请求与响应 Schema、HTTP 状态码与错误映射，以及从响应和日志中排除秘密的方法。由于目前没有服务端，不提供可执行的 `curl` 请求。
