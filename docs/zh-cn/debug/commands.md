# Debug Terminal 命令参考

以下命令由 `internal/terminal` 解析。输出示例为代表性文本，任务 ID、PID 和路径会因环境而异。

## 基础命令

| 输入 | 用途 | 输出示例 |
|---|---|---|
| `help` | 显示命令帮助；无参数 | `Commands:` 和用法列表 |
| `clear` | 清屏；非交互模式输出空行 | 清空屏幕 |
| `version` | 显示 CLE 版本 | `ChesLaunchEngine dev` |
| `version list` | 最多显示 25 个清单版本 | `1.21.1 release ...` |
| `version inspect 1.21.1` | 解析并显示版本元数据 | ID、类型、主类、Java 主版本、Assets、库数量 |
| `version resolve 1.21.1` | 解析元数据并报告成功 | 版本摘要及解析成功消息 |
| `exit` / `quit` | 退出调试台并清理引擎资源 | 返回调用的 Shell |

## 启动任务和进程

| 命令 | 用途 | 输出示例 |
|---|---|---|
| `launch [--config <request.json>]` | 异步启动默认或指定请求 | 任务 ID、版本、Java、账户类型、阶段和待定 PID |
| `tasks` | 列出启动任务 | `<task-id> version=1.21.1 stage=running pid=1234 exit=pending` |
| `status [task-id]` | 查看指定任务或所有任务状态 | 任务 ID、版本、阶段、PID、退出码 |
| `cancel <task-id>` | 取消启动任务，通常用于解析/下载阶段 | 成功后任务进入 cancelled |
| `process [list]` | 列出受管理的 Minecraft 进程 | `pid=1234 task=... status=running exit=pending` |
| `process inspect <pid>` | 查看受管理进程；PID 必须为正整数 | 进程摘要或未找到错误 |
| `stop <pid> [--force]` | 温和停止进程；强制终止须明确指定 | `Graceful stop requested for process 1234.` |

## 版本、Java、下载和日志

| 命令 | 用途 | 输出示例 |
|---|---|---|
| `java list` | 列出检测到的 Java | 版本、主版本、架构和路径 |
| `java detect` | 按配置或默认方式检测 Java | Java 信息或未找到提示 |
| `java inspect <java-path>` | 检查指定 Java 程序 | Java 版本、主版本、架构、路径 |
| `download status` | 显示正在下载任务的计数 | `task=... downloads=12/100 stage=downloading` |
| `download list` | 显示近期下载事件 | `[download_progress task=...] Downloaded ...` |
| `download cancel <task-id>` | 取消该任务的下载阶段 | 成功后取消任务 |
| `logs --task <id> --limit 20` | 查看任务近期事件；默认 100，最多 2000 | `[game_stdout task=... pid=1234 INFO] ...` |
| `logs --process <pid>` | 按进程筛选日志 | 匹配的 stdout/stderr 和生命周期事件 |
| `logs --task <id> --follow` | 显示历史并开启实时观察 | `Following matching events; use unwatch to stop.` |
| `watch` | 实时观察全部任务 | `Watching events from all tasks; use unwatch to stop.` |
| `watch <task-id>` / `watch <pid>` | 按任务或进程筛选实时事件 | `Watching events for ...` |
| `unwatch` | 停止显示实时事件 | `Stopped displaying watched events.` |

## 调试信息和账户

| 命令 | 用途 | 输出示例 |
|---|---|---|
| `debug config` | 显示已加载 CLE 配置 | 格式化 JSON |
| `debug info <task-id>` | 显示阶段、Java、PID、下载计数 | `task=... version=... stage=running ...` |
| `debug arguments <task-id>` | 显示脱敏后的 Java/JVM/游戏参数 | Java 路径、JVM 参数、主类、工作目录 |
| `debug classpath <task-id>` | 显示 Classpath 条目 | `Classpath entries: 70` 及路径 |
| `debug environment <task-id>` | 显示脱敏后的环境变量覆盖 | 排序的 `KEY=value` |
| `account list` | 列出已保存账户及活动账户标记 | `<uuid> offline Alex (active)` |
| `account add offline <name> [uuid]` | 创建离线账户；UUID 可选且须为标准格式 | 账户名、ID、UUID |
| `account add yggdrasil <auth-server-url> <username>` | 添加 Yggdrasil 账户；密码通过隐藏交互提示读取，不得作为参数；要求 HTTPS | 创建账户及 UUID |
| `account add microsoft` | 启动 OAuth Device Code 登录；显示验证网址和用户代码并轮询至完成；需配置 Client ID | 创建账户及 UUID |
| `account inspect <id>` | 查看非敏感账户元数据 | 类型、用户名、显示名、UUID、时间 |
| `account profile <id>` | 查看游戏资料 | 名称、ID、属性数量 |
| `account validate <id>` | 本地校验离线资料，或远程验证 Yggdrasil 会话 | `Account profile is valid.` |
| `account switch <id>` | 设为活动账户 | `Active account set to <id>` |
| `account refresh <id>` | 刷新 Yggdrasil 会话；离线账户不支持 | `Account refreshed: <id>` |
| `account logout <id>` | 使 Yggdrasil 会话失效并删除凭据；资料保留 | `Account session logged out.` |
| `account remove <id>` | 删除资料及对应 Yggdrasil 凭据 | `Account removed.` |

Yggdrasil Token 使用操作系统凭据库（Windows Credential Manager、macOS
Keychain 或 Linux Secret Service）；Linux 需要可用的 Secret Service 会话。
Microsoft 账户使用 OAuth Device Code 流程，其 Refresh Token、OAuth Access
Token 与 Minecraft Access Token 同样通过操作系统凭据库按账户 ID 隔离保存。
Microsoft 账户需要配置 Microsoft 公共客户端应用 ID（`account.microsoft.clientId`
或 `CLE_MICROSOFT_CLIENT_ID`），CLE 不内置 Client ID。

`cle-debug --command "help"` 执行单条命令后退出；单次 `launch` 会等待任务完成并输出事件。本程序不是机器可读 JSON 协议；正式 CLI 的 JSON 响应请用 `cle --json`。
