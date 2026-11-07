# Debug Terminal 命令參考

以下命令由 `internal/terminal` 解析。輸出範例為代表性文字，工作 ID、PID 和路徑會依環境而異。

## 基本命令

| 輸入 | 用途 | 輸出範例 |
|---|---|---|
| `help` | 顯示命令說明；無參數 | `Commands:` 和用法清單 |
| `clear` | 清除畫面；非互動模式輸出空行 | 清除畫面 |
| `version` | 顯示 CLE 版本 | `ChesLaunchEngine dev` |
| `version list` | 最多顯示 25 個清單版本 | `1.21.1 release ...` |
| `version inspect 1.21.1` | 解析並顯示版本中繼資料 | ID、類型、主類別、Java 主版本、Assets、程式庫數量 |
| `version resolve 1.21.1` | 解析中繼資料並回報成功 | 版本摘要及解析成功訊息 |
| `exit` / `quit` | 離開除錯終端機並清理引擎資源 | 返回呼叫端 Shell |

## 啟動工作與處理程序

| 命令 | 用途 | 輸出範例 |
|---|---|---|
| `launch [--config <request.json>]` | 非同步啟動預設或指定要求 | 工作 ID、版本、Java、帳戶類型、階段和待定 PID |
| `tasks` | 列出啟動工作 | `<task-id> version=1.21.1 stage=running pid=1234 exit=pending` |
| `status [task-id]` | 查看指定工作或所有工作狀態 | 工作 ID、版本、階段、PID、結束碼 |
| `cancel <task-id>` | 取消啟動工作，通常用於解析／下載階段 | 成功後工作進入 cancelled |
| `process [list]` | 列出受管理的 Minecraft 處理程序 | `pid=1234 task=... status=running exit=pending` |
| `process inspect <pid>` | 查看受管理處理程序；PID 必須為正整數 | 處理程序摘要或找不到錯誤 |
| `stop <pid> [--force]` | 溫和停止處理程序；強制終止須明確指定 | `Graceful stop requested for process 1234.` |

## 版本、Java、下載與記錄

| 命令 | 用途 | 輸出範例 |
|---|---|---|
| `java list` | 列出偵測到的 Java | 版本、主版本、架構和路徑 |
| `java detect` | 依設定或預設方式偵測 Java | Java 資訊或找不到提示 |
| `java inspect <java-path>` | 檢查指定的 Java 程式 | Java 版本、主版本、架構、路徑 |
| `download status` | 顯示進行中工作的下載計數 | `task=... downloads=12/100 stage=downloading` |
| `download list` | 顯示近期下載事件 | `[download_progress task=...] Downloaded ...` |
| `download cancel <task-id>` | 取消該工作的下載階段 | 成功後取消工作 |
| `logs --task <id> --limit 20` | 查看工作近期事件；預設 100，最多 2000 | `[game_stdout task=... pid=1234 INFO] ...` |
| `logs --process <pid>` | 依處理程序篩選記錄 | 符合的 stdout/stderr 和生命週期事件 |
| `logs --task <id> --follow` | 顯示歷史記錄並開始即時監看 | `Following matching events; use unwatch to stop.` |
| `watch` | 即時監看所有工作 | `Watching events from all tasks; use unwatch to stop.` |
| `watch <task-id>` / `watch <pid>` | 依工作或處理程序篩選即時事件 | `Watching events for ...` |
| `unwatch` | 停止顯示即時事件 | `Stopped displaying watched events.` |

## 除錯資訊與帳戶

| 命令 | 用途 | 輸出範例 |
|---|---|---|
| `debug config` | 顯示已載入的 CLE 設定 | 格式化 JSON |
| `debug info <task-id>` | 顯示階段、Java、PID、下載計數 | `task=... version=... stage=running ...` |
| `debug arguments <task-id>` | 顯示已遮蔽的 Java/JVM/遊戲參數 | Java 路徑、JVM 參數、主類別、工作目錄 |
| `debug classpath <task-id>` | 顯示 Classpath 項目 | `Classpath entries: 70` 和路徑 |
| `debug environment <task-id>` | 顯示已遮蔽的環境變數覆寫 | 排序後的 `KEY=value` |
| `account list` | 列出已儲存帳戶並標示作用中帳戶 | `<uuid> offline Alex (active)` |
| `account add offline <name> [uuid]` | 建立離線帳戶；UUID 選填且須為標準格式 | 帳戶名稱、ID、UUID |
| `account add yggdrasil <auth-server-url> <username>` | 新增 Yggdrasil 帳戶；密碼透過隱藏互動提示讀取，不得作為參數；要求 HTTPS | 建立帳戶及 UUID |
| `account add microsoft` | 啟動 OAuth Device Code 登入；顯示驗證網址與使用者代碼並輪詢至完成；需設定 Client ID | 建立帳戶及 UUID |
| `account inspect <id>` | 查看非敏感帳戶中繼資料 | 類型、使用者名稱、顯示名稱、UUID、時間 |
| `account profile <id>` | 查看遊戲資料 | 名稱、ID、屬性數量 |
| `account validate <id>` | 在本機驗證離線資料，或遠端驗證 Yggdrasil 工作階段 | `Account profile is valid.` |
| `account switch <id>` | 設為作用中帳戶 | `Active account set to <id>` |
| `account refresh <id>` | 更新 Yggdrasil 工作階段；離線帳戶不支援 | `Account refreshed: <id>` |
| `account logout <id>` | 使 Yggdrasil 工作階段失效並刪除憑證；資料仍保留 | `Account session logged out.` |
| `account remove <id>` | 刪除資料及對應的 Yggdrasil 憑證 | `Account removed.` |

Yggdrasil Token 使用作業系統憑證庫（Windows Credential Manager、macOS
Keychain 或 Linux Secret Service）；Linux 需要可用的 Secret Service 工作階段。
Microsoft 帳戶使用 OAuth Device Code 流程，其 Refresh Token、OAuth Access
Token 與 Minecraft Access Token 同樣透過作業系統憑證庫按帳戶 ID 隔離儲存。
Microsoft 帳戶需設定 Microsoft 公開用戶端應用程式 ID（`account.microsoft.clientId`
或 `CLE_MICROSOFT_CLIENT_ID`），CLE 不內建 Client ID。

`cle-debug --command "help"` 執行單一命令後退出；單次 `launch` 會等待工作完成並輸出事件。本程式不是機器可讀 JSON 協定；正式 CLI 的 JSON 回應請使用 `cle --json`。
