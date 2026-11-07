# CLI JSON 輸出

在正式 CLI 命令中加入 `--json`：

```sh
cle --json -version
cle --json -check-config ./cle.json
cle --json -config ./cle.json -launch ./launch.json
```

CLI 會向 stdout 寫入一行 JSON：

```json
{"success":true,"data":{"command":"version","version":"dev"},"error":null}
```

`data` 會依命令而異。啟動回應包含 `{"command":"launch","task":{...}}`，工作欄位遵循 `pkg/models.LaunchTask` 的 JSON 標籤。

失敗回應使用相同結構和穩定錯誤碼：

```json
{"success":false,"data":null,"error":{"code":"INVALID_CONFIGURATION","message":"decode configuration JSON"}}
```

引擎錯誤使用其定義的錯誤碼；其他錯誤使用 `CLI_ERROR`。錯誤訊息會經過遮蔽，不包含堆疊、憑證或 Token。呼叫端應同時檢查 `success` 和程序結束碼。

JSON 模式不會將主控台記錄或人類可讀的啟動訊息寫入 stdout；已設定的檔案記錄仍可保留。編碼診斷資訊寫入 stderr。此模式每個命令只回應一次，不提供串流進度；即時工作和下載事件請使用 `cle-debug`。

PowerShell：

```powershell
$response = cle --json -version | ConvertFrom-Json
if (-not $response.success) { throw $response.error.message }
$response.data.version
```

POSIX Shell：

```sh
response="$(cle --json -version)" || exit $?
printf '%s\n' "$response" | jq -r '.data.version'
```

`jq` 是可選的 JSON 處理工具，不是 CLE 執行階段相依項目。
