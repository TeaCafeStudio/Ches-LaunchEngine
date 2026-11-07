# 启动任务与取消

**启动任务**代表解析、校验、下载、准备和启动的完整流程。`tasks` 列出工作流；`status <task-id>` 显示阶段，以及可用的 PID 和退出码。常见阶段：`resolving`、`validating`、`downloading`、`preparing`、`launching`、`running`，终态包括 `failed`、`cancelled`、`stopped` 和 `exited`。

`cancel <task-id>` 取消启动操作，尤其适用于元数据或文件仍在准备时；它不是停止正在运行游戏的命令。`download cancel <task-id>` 针对该任务的下载阶段。

任务 ID 是 CLE 工作流标识符，不是操作系统进程 ID。使用 `process list` 查看与任务关联的游戏进程。

任务失败时，`status <task-id>` 显示错误码、消息、失败阶段、近期任务事件和可能的排查提示。Minecraft 成功启动后再退出时，可通过进程退出事件和退出码区分于启动前失败。
