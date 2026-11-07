# 啟動要求 JSON

啟動要求描述一次 Minecraft 啟動，不是 CLE 執行設定，也不使用 CLE 設定 Schema 驗證。

```json
{
  "versionId": "1.21.1",
  "gameDirectory": "./game",
  "javaPath": "java",
  "account": {"name": "OfflinePlayer", "type": "offline"},
  "loader": {"type": "vanilla"},
  "argumentMode": "auto",
  "window": {"width": 1280, "height": 720}
}
```

| 欄位 | 型別 | 必填／預設 | 說明 |
|---|---|---|---|
| `versionId` | string | 引擎必填 | Minecraft 版本 ID |
| `versionDirectory` | string | 選填 | 版本中繼資料/JAR 目錄；否則由遊戲目錄或設定推導 |
| `gameDirectory` | string | 設定未提供時必填 | 遊戲資料根目錄 |
| `javaPath` | string | 選填 | Java 程式；預設使用設定值或自動偵測 |
| `javaArguments` | string array | 選填 | 額外 JVM 參數 |
| `gameArguments` | string array | 選填 | 額外遊戲參數 |
| `account` | object | 設定未提供時必填 | 內嵌離線身分或已儲存帳戶 `id`（離線或 Yggdrasil） |
| `loader` | object | 選填；預設 Vanilla | 目前 `type` 只支援 `vanilla`；`version` 為後續載入器預留 |
| `downloadOptions` | object | 選填 | `offline` 布林值；`concurrency` 為 0/預設或 1–64；`retries` 為 0/預設或 0–10 |
| `window` | object | 選填 | `width`、`height` 整數和 `fullscreen` 布林值 |
| `environment` | string map | 選填 | 子處理程序環境變數覆寫 |
| `workingDirectory` | string | 選填 | 必須是現存目錄；否則使用遊戲目錄 |
| `argumentMode` | string | 選填；預設 `auto` | `auto`、`legacy` 或 `modern` |

帳戶模型也接受 `id`、`name`、`uuid`、`displayName`、`profile`、`type`、`authServer`、`credentialRef`、`xuid`、`userType` 和時間戳記。可使用有效的內嵌離線身分或已儲存帳戶 `id`。Yggdrasil 啟動必須使用已儲存帳戶 ID，讓 CLE 從作業系統憑證庫讀取並驗證工作階段。啟動要求不得包含 Token 或密碼。

使用已儲存帳戶的範例：

```json
{
  "versionId": "1.21.1",
  "gameDirectory": "C:\\Games\\Minecraft",
  "javaPath": "C:\\Program Files\\Java\\jdk-21\\bin\\java.exe",
  "account": {"id": "replace-with-id-from-account-list"},
  "loader": {"type": "vanilla"}
}
```

Linux/macOS 的 `gameDirectory` 範例：`/home/alex/.minecraft` 或 `/Users/alex/Library/Application Support/minecraft`。若出現 `unknown field "accountId"`，請改用實際欄位 `account.id`。解碼器會拒絕未知欄位和多個 JSON 值。
