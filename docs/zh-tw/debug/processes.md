# 遊戲處理程序

`process list` 和 `process inspect <pid>` 只會回報由此引擎啟動並追蹤的 Minecraft 處理程序。PID 是作業系統處理程序 ID，不是 CLE 啟動工作 ID。

要求溫和停止：

```text
stop 1234
```

只有明確需要時才強制終止：

```text
stop 1234 --force
```

類 Unix 系統會傳送中斷訊號。Windows 會向 Java 處理程序的頂層視窗傳送關閉要求；若沒有可用視窗，要求可能失敗，此時才考慮明確執行強制停止。

`process inspect` 和 `process list` 會在已知時顯示狀態與結束碼。`logs --process 1234` 依處理程序篩選近期事件。除錯終端機不會控制無關的 Java 處理程序。
