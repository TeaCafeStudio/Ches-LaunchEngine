# Debug Terminal 除錯終端機

`cle-debug` 是獨立的開發者程式，透過公開的 `core.DebugEngine` 和 SDK 工廠使用同一引擎，不含第二套啟動器；主程式 `cle` 不會匯入終端機套件。

```sh
cle-debug
cle-debug --config ./cle.json
cle-debug --config ./launch.json
```

`--config` 可接收 CLE 設定或啟動要求；後者會成為 `launch` 的預設要求。`--launch-config` 可明確指定預設啟動要求。使用 `cle-debug --config ./launch.json --command "help"` 執行單一命令。`--command "launch"` 會持續執行並輸出事件，直到工作完成、失敗或取消；Ctrl+C 會取消目前工作。

除錯終端機訂閱有界 Core 事件，區分引擎記錄、遊戲 stdout/stderr、下載進度、工作狀態和處理程序生命週期。`watch` 控制即時輸出，`logs` 查詢記憶體中的近期事件。敏感參數會遮蔽。

在終端機輸入 `help` 查看命令。另請參閱[命令參考](commands.md)、[啟動](launch.md)、[工作](tasks.md)、[處理程序](processes.md)和[疑難排解](troubleshooting.md)。
