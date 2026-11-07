# 啟動工作與取消

**啟動工作**代表解析、驗證、下載、準備和啟動的完整流程。`tasks` 列出工作流程；`status <task-id>` 顯示目前階段，以及可用的 PID 和結束碼。常見階段：`resolving`、`validating`、`downloading`、`preparing`、`launching`、`running`；終止狀態包括 `failed`、`cancelled`、`stopped` 和 `exited`。

`cancel <task-id>` 會取消啟動作業，特別適用於中繼資料或檔案仍在準備時；它不是停止執行中遊戲的命令。`download cancel <task-id>` 會針對該工作的下載階段。

工作 ID 是 CLE 工作流程識別碼，不是作業系統處理程序 ID。使用 `process list` 查看與工作相關聯的遊戲處理程序。

工作失敗時，`status <task-id>` 會顯示錯誤碼、訊息、失敗階段、近期工作事件和可能的診斷提示。Minecraft 成功啟動後才結束時，可透過處理程序結束事件和結束碼與啟動前失敗區分。
