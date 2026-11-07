# 規劃中的 HTTP 端點

**Planned / Not Implemented（規劃中／尚未實作）**。以下路由均未註冊，目前不可呼叫：

- `GET /api/v1/accounts`
- `POST /api/v1/accounts`
- `GET /api/v1/accounts/{id}`
- `POST /api/v1/accounts/{id}/validate`
- `POST /api/v1/accounts/{id}/refresh`
- `POST /api/v1/accounts/{id}/logout`
- `DELETE /api/v1/accounts/{id}`
- `POST /api/v1/auth/microsoft/start`
- `GET /api/v1/auth/microsoft/callback`

實作前必須定義呼叫端驗證／授權、OAuth 回呼策略、要求與回應 Schema、HTTP 狀態碼與錯誤對應，以及從回應和記錄中排除秘密的方法。目前沒有伺服器可接收要求，因此不提供可執行的 `curl` 範例。
