# CLE 開發者文件（繁體中文）

其他語言：[简体中文](../zh-cn/README.md) · [English](../en-us/README.md) · [繁體中文](README.md)

ChesLaunchEngine（CLE）是以 Go 撰寫、面向 Minecraft Java Edition 的引擎，提供正式命令列程式 `cle`、獨立除錯終端機 `cle-debug` 和 Go SDK。

## 文件目錄

- [快速入門](getting-started.md) · [安裝與建置](installation.md)
- [CLI 概覽](cli/overview.md) · [命令參考](cli/commands.md) · [JSON 輸出](cli/json-output.md)
- [除錯終端機概覽](debug/overview.md) · [命令參考](debug/commands.md) · [啟動](debug/launch.md) · [工作](debug/tasks.md) · [處理程序](debug/processes.md) · [疑難排解](debug/troubleshooting.md)
- [設定概覽](configuration/overview.md) · [啟動要求](configuration/launch-config.md) · [帳戶](configuration/accounts.md) · [設定 Schema](configuration/schema.md)
- [HTTP API 狀態](api/overview.md) · [JSON 慣例](api/json-format.md) · [規劃中的端點](api/endpoints.md)
- [Go SDK](sdk/go.md) · [建置與測試](development/build.md) · [架構](development/architecture.md) · [貢獻指南](development/contribution.md)

離線帳戶和 Yggdrasil 帳戶已實作。Yggdrasil 啟動使用 SHA-256 固定驗證的
Authlib Injector 1.2.8 Java Agent；尚未驗證真實驗證、進入 Minecraft 主畫面或
作業系統憑證庫執行行為。Microsoft 驗證、非 Go SDK、模組載入器和 HTTP API
尚未實作。命令、JSON 鍵名、路徑及程式碼識別字維持原樣。
