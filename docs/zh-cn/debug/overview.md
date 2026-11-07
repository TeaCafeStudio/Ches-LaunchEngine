# Debug Terminal 调试台

`cle-debug` 是独立的开发者程序，通过公开的 `core.DebugEngine` 和 SDK 工厂使用同一引擎，不包含第二套启动器；主程序 `cle` 不导入终端包。

```sh
cle-debug
cle-debug --config ./cle.json
cle-debug --config ./launch.json
```

`--config` 可接收 CLE 配置或启动请求；后者会成为 `launch` 的默认请求。`--launch-config` 可明确指定默认启动请求。用 `cle-debug --config ./launch.json --command "help"` 执行单条命令。`--command "launch"` 会持续运行并输出事件，直到任务完成、失败或取消；Ctrl+C 会取消活动任务。

调试台订阅有界 Core 事件，区分引擎日志、游戏 stdout/stderr、下载进度、任务状态和进程生命周期。`watch` 控制实时输出，`logs` 查询内存中的近期事件。敏感参数会脱敏。

在调试台输入 `help` 查看命令。另见[命令参考](commands.md)、[启动](launch.md)、[任务](tasks.md)、[进程](processes.md)和[故障排除](troubleshooting.md)。
