# 架構

```text
cle CLI -----------------------> pkg/sdk -> core.MinecraftEngine -> engine
cle-debug -> internal/terminal -> pkg/sdk -> core.DebugEngine ----> engine
```

正式 CLI Adapter 位於 `internal/cli`；`cmd/cle` 委派給它，不相依於 `internal/terminal`。獨立入口 `cmd/cle-debug` 負責互動式初始化並匯入終端機。兩者使用相同 SDK 工廠和 Vanilla 引擎實作，不存在重複啟動器。

`pkg/core` 和 `pkg/models` 定義共用介面與資料；`pkg/sdk` 建立引擎；`internal/config` 負責嚴格解析 CLE 設定與啟動要求；`internal/terminal` 透過公開 Debug Core 介面實作命令和呈現。事件來自引擎既有的有界事件匯流排。

`pkg/accounts` 包含離線和 Yggdrasil Provider 及帳戶管理器。Yggdrasil 工作階段 Token 與帳戶資料 JSON 分開，儲存在作業系統憑證庫。CLI 和除錯終端機只向 Core 傳遞已儲存帳戶 ID；Core 會在版本安裝和程序建立前選擇 Provider 並解析有效工作階段。同一帳戶的更新、驗證、啟動工作階段解析、登出和刪除會序列化執行。

Yggdrasil JVM 參數包含來自官方發佈來源、經過固定 SHA-256 驗證的 Authlib Injector 1.2.8 Agent。Minecraft 用戶端的工作階段要求獨立於 CLE 登入要求，因此需要此注入。模擬伺服器和參數位置測試涵蓋此整合，但尚未驗證真實驗證、實機啟動或 Windows/macOS/Linux 憑證庫執行行為。Microsoft 驗證尚未實作。

目前沒有 HTTP API 套件或伺服器。`api.enabled` 是預留設定，不會啟用監聽器。
