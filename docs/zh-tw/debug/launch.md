# 從除錯終端機啟動遊戲

啟動時指定預設要求：

```sh
cle-debug --config ./launch.json
```

進入終端機後輸入 `launch`；也可指定要求：`launch --config ./examples/launch/vanilla.json`。

終端機會顯示啟動工作 ID、版本、Java 路徑、帳戶類型、遊戲目錄、載入器、目前階段和可用的 PID，並輸出事件，例如：

```text
[task_status task=cle-... INFO] Launch task status changed to downloading
[download_progress task=cle-... INFO] Downloaded ... (12/100 files)
[process_started task=cle-... pid=1234 INFO] Minecraft process started
[game_stdout task=cle-... pid=1234 INFO] ...
```

使用 `watch <task-id>` 或 `logs --task <task-id>` 篩選輸出。`debug arguments <task-id>` 會顯示已遮蔽的解析參數。請勿將原始憑證寫入啟動要求。

## 使用已儲存的帳戶

使用 `account add offline Alex` 建立離線帳戶，或使用 `account add yggdrasil <auth-server-url> <username>` 新增 Yggdrasil 帳戶。後者會隱藏密碼輸入並要求 HTTPS。透過 `account list` 查看後，在啟動要求中指定 `{"account":{"id":"<stored-id>"}}`。若要求未指定帳戶身分，則會使用目前作用中的已儲存帳戶。內嵌格式 `{"account":{"name":"Alex","type":"offline"}}` 仍可使用。

Yggdrasil 啟動會從作業系統憑證庫讀取工作階段；Token 不得寫入啟動 JSON。
帳戶的 Yggdrasil API 基礎網址會以 JVM Agent 參數傳給 Authlib Injector，使
Minecraft 用戶端本身的工作階段要求也使用第三方服務。首次啟動會下載固定版本且
經過 SHA-256 驗證的 Authlib Injector 1.2.8；離線模式要求遊戲目錄快取中已有通過
驗證的檔案。相容性取決於 Minecraft/Authlib 版本和伺服器；尚未驗證真實驗證或實機
啟動。離線資料只是本機身分，不能證明帳戶所有權，也無法通過正版線上多人伺服器驗證。
