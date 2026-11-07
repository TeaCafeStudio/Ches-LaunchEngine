# 疑難排解

## 找不到 Java 或版本不相容

執行 `java detect` 或 `java list`。在啟動要求設定 `javaPath`，或在 CLE 設定設定 `java.path`。Minecraft 1.21.1 需要 Java 21。

## 中繼資料或下載失敗

檢查網路、磁碟空間、版本 ID、遊戲目錄和版本目錄。`download status` 顯示作用中工作的下載計數；`download list` 顯示近期進度事件。

## 啟動工作失敗

執行 `tasks`，再執行 `status <task-id>`。檢查錯誤碼和失敗階段；使用 `logs --task <task-id>` 和 `debug info <task-id>` 查看上下文。引擎會遮蔽已識別的 Token／密碼指派內容。

## 遊戲仍在執行，需要關閉

使用 `process list` 查詢 PID，再執行 `stop <pid>`。工作 ID 不是 PID。只有不適合使用溫和停止時，才使用 `stop <pid> --force`。

## 離線帳戶無法加入伺服器

離線帳戶不是已驗證的 Microsoft/Mojang 帳戶，無法加入正版線上模式伺服器。線上帳戶 Provider 尚未實作。

## 終端機無法互動

管線輸入會逐行視為一個命令，並在 EOF 時退出。可使用 `cle-debug --command "help"` 在沒有互動式 TTY 時執行單一命令。
