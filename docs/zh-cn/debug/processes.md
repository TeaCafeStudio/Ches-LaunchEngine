# 游戏进程

`process list` 和 `process inspect <pid>` 只报告由此引擎启动并跟踪的 Minecraft 进程。PID 是操作系统进程 ID，不是 CLE 启动任务 ID。

请求温和停止：

```text
stop 1234
```

只有明确需要时才强制终止：

```text
stop 1234 --force
```

类 Unix 系统会发送中断信号。Windows 会向 Java 进程的顶层窗口发送关闭请求；若没有可用窗口，请求可能失败，此时才考虑明确执行强制停止。

`process inspect` 和 `process list` 会在已知时显示状态与退出码。`logs --process 1234` 按进程筛选近期事件。调试台不会管理无关的 Java 进程。
