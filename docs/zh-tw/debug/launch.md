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

## 使用已儲存的離線帳戶

使用 `account add offline Alex` 建立帳戶，並以 `account list` 查看。啟動要求可包含 `{"account":{"id":"<stored-id>"}}`。若要求未指定帳戶身分，則會使用目前作用中的離線帳戶。既有的內嵌格式 `{"account":{"name":"Alex","type":"offline"}}` 仍可使用。

離線資料只是本機身分，不能證明帳戶所有權，也無法通過正版線上多人伺服器驗證。
