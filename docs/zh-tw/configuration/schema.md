# CLE 設定 Schema

權威 JSON Schema：[`configs/cle.schema.json`](../../../configs/cle.schema.json)。Go 解碼器也會套用預設值並執行執行階段驗證。根層級 `version` 為必填且必須是 `1`；未知欄位和多餘 JSON 值都會報錯。

| JSON 路徑 | 型別 | 必填 | 預設值／限制 |
|---|---|---|---|
| `version` | integer | 是 | 必須為 `1` |
| `paths.gameDirectory` | string | 否 | 空；可由啟動要求提供 |
| `paths.versionsDirectory` | string | 否 | 空；由遊戲目錄推導 |
| `launch.argumentMode` | string | 否 | `auto`；允許 `auto`、`legacy`、`modern` |
| `launch.timeoutSeconds` | integer | 否 | `3600`；必須為正數 |
| `download.concurrency` | integer | 否 | `8`；1–64 |
| `download.retries` | integer | 否 | `3`；0–10 |
| `download.timeoutSeconds` | integer | 否 | `300`；必須為正數 |
| `download.offline` | boolean | 否 | `false` |
| `java.path` | string | 否 | 空；自動偵測 |
| `account.defaultProvider` | string | 否 | 空；目前只支援 `offline` |
| `account.credentialStore` | string | 否 | 空；非空值目前不支援 |
| `account.storePath` | string | 否 | 空；使用作業系統使用者設定路徑；不得全為空白 |
| `logging.level` | string | 否 | `INFO`；允許 `DEBUG`、`INFO`、`WARN`、`ERROR` |
| `logging.console` | boolean | 否 | `true`；記錄寫入 stderr |
| `logging.directory` | string | 否 | 空；設定後必須同時提供 `instanceId` |
| `logging.instanceId` | string | 條件必填 | 使用記錄目錄時必填 |
| `api.enabled` | boolean | 否 | `false`；HTTP API 尚未實作 |
| `api.address` | string | 否 | `127.0.0.1:8765`；啟用設定時要求填寫，但不會啟動服務 |
| `debugTerminal.enabled` | boolean | 否 | `true` |
| `debugTerminal.historyLimit` | integer | 否 | `1000`；0–10000 |

`cle -check-config <file>` 驗證 CLE 設定，不驗證啟動要求。Schema 中的預設值僅供說明；執行時預設值由 Go 的 `Default` 函式設定。

最小設定：`{"version":1}`。

完整設定：

```json
{
  "version": 1,
  "paths": {"gameDirectory": "./game", "versionsDirectory": "./game/versions"},
  "launch": {"argumentMode": "auto", "timeoutSeconds": 3600},
  "download": {"concurrency": 8, "retries": 3, "timeoutSeconds": 300, "offline": false},
  "java": {"path": "java"},
  "account": {"defaultProvider": "offline", "storePath": "./accounts.json"},
  "logging": {"level": "INFO", "console": true},
  "api": {"enabled": false, "address": "127.0.0.1:8765"},
  "debugTerminal": {"enabled": true, "historyLimit": 1000}
}
```

Windows 路徑必須在 JSON 中跳脫，例如 `"C:\\Games\\CLE"`。Linux/macOS 使用 `~` 時必須由呼叫端展開；CLE 不保證執行 Shell 展開。錯誤範例 `{"version":1,"download":{"concurrency":0}}` 會遭拒絕，並行數必須為 1–64。
