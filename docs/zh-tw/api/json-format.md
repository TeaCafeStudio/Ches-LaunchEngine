# 未來 HTTP API 的 JSON 慣例

**Planned / Not Implemented（規劃中／尚未實作）**。此頁僅記錄文件大綱，不定義可呼叫的協定。

實作 HTTP API 時，文件必須明確說明 Content-Type、要求／回應 Schema、HTTP 狀態碼、驗證與授權、遮蔽規則和穩定錯誤碼。目前沒有 HTTP 回應封裝格式。

正式 CLI 已個別實作 `--json`，其回應為 `{ "success": boolean, "data": value|null, "error": object|null }`。請勿假設 HTTP API 也已使用此格式。
