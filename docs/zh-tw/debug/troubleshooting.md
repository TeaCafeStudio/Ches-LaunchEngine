# 疑難排解

## 找不到 Java 或版本不相容

執行 `java detect` 或 `java list`。在啟動要求設定 `javaPath`，或在 CLE 設定設定 `java.path`。Minecraft 1.21.1 需要 Java 21。

## 中繼資料或下載失敗

檢查網路、磁碟空間、版本 ID、遊戲目錄和版本目錄。`download status` 顯示作用中工作的下載計數；`download list` 顯示近期進度事件。

## 啟動工作失敗

執行 `tasks`，再執行 `status <task-id>`。檢查錯誤碼和失敗階段；使用 `logs --task <task-id>` 和 `debug info <task-id>` 查看上下文。引擎會遮蔽已識別的 Token／密碼指派內容。

遇到 `ACCOUNT_NOT_FOUND` 時，請確認啟動要求中的帳戶 ID 與 `account list` 顯示的 ID
一致。遇到 `SESSION_INVALID` 或 `SESSION_REFRESH_FAILED` 時，請檢查 Yggdrasil API
基礎網址和作業系統憑證庫；不要將 Token 或密碼寫入要求。遇到
`AUTHLIB_INJECTOR_UNAVAILABLE` 或下載／完整性錯誤時，請在網路可用時重試。離線模式
要求遊戲庫快取中已有通過驗證的 Authlib Injector 1.2.8 JAR。相容性也取決於
Minecraft/Authlib 版本和服務商。

## 遊戲仍在執行，需要關閉

使用 `process list` 查詢 PID，再執行 `stop <pid>`。工作 ID 不是 PID。只有不適合使用溫和停止時，才使用 `stop <pid> --force`。

## 離線帳戶無法加入伺服器

離線帳戶不是已驗證的 Microsoft/Mojang 帳戶，無法加入正版線上模式伺服器。
Yggdrasil Provider 已實作，但真實服務驗證與遊戲啟動尚未驗證；Microsoft 驗證尚未實作。

## 終端機無法互動

管線輸入會逐行視為一個命令，並在 EOF 時退出。可使用 `cle-debug --command "help"` 在沒有互動式 TTY 時執行單一命令。
