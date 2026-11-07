# 架構

```text
cle CLI -----------------------> pkg/sdk -> core.MinecraftEngine -> engine
cle-debug -> internal/terminal -> pkg/sdk -> core.DebugEngine ----> engine
```

正式 CLI Adapter 位於 `internal/cli`；`cmd/cle` 委派給它，不相依於 `internal/terminal`。獨立入口 `cmd/cle-debug` 負責互動式初始化並匯入終端機。兩者使用相同 SDK 工廠和 Vanilla 引擎實作，不存在重複啟動器。

`pkg/core` 和 `pkg/models` 定義共用介面與資料；`pkg/sdk` 建立引擎；`internal/config` 負責嚴格解析 CLE 設定與啟動要求；`internal/terminal` 透過公開 Debug Core 介面實作命令和呈現。事件來自引擎既有的有界事件匯流排。

目前沒有 HTTP API 套件或伺服器。`api.enabled` 是預留設定，不會啟用監聽器。Microsoft/Yggdrasil Provider 尚未實作。
