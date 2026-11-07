# 故障排除

## 找不到 Java 或版本不兼容

运行 `java detect` 或 `java list`。在启动请求设置 `javaPath`，或在 CLE 配置设置 `java.path`。Minecraft 1.21.1 需要 Java 21。

## 元数据或下载失败

检查网络、磁盘空间、版本 ID、游戏目录和版本目录。`download status` 显示活动任务的下载计数；`download list` 显示近期进度事件。

## 启动任务失败

运行 `tasks`，再运行 `status <task-id>`。检查错误码和失败阶段；使用 `logs --task <task-id>` 和 `debug info <task-id>` 查看上下文。引擎会对已识别的 Token/密码赋值做脱敏。

遇到 `ACCOUNT_NOT_FOUND` 时，确认启动请求中的账户 ID 与 `account list` 显示的 ID 一致。
遇到 `SESSION_INVALID` 或 `SESSION_REFRESH_FAILED` 时，检查 Yggdrasil API 基础 URL 和
操作系统凭据库；不要将 Token 或密码写入请求。遇到 `AUTHLIB_INJECTOR_UNAVAILABLE`
或下载/完整性错误时，请在网络可用时重试。离线模式要求游戏库缓存中已有验证通过的
Authlib Injector 1.2.8 JAR。兼容性也取决于 Minecraft/Authlib 版本和服务商。

## 游戏正在运行，需要关闭

用 `process list` 查 PID，再运行 `stop <pid>`。任务 ID 不是 PID。只有温和停止不适用时才用 `stop <pid> --force`。

## 离线账户无法进入服务器

离线账户不是已认证的 Microsoft/Mojang 账户，无法加入正版在线模式服务器。Yggdrasil 账户须使用对应第三方服务器的认证与兼容策略。Microsoft 认证尚未实现。

## 终端无法交互

管道输入按每行一条命令处理，并在 EOF 时退出。可用 `cle-debug --command "help"` 在无交互 TTY 下执行单条命令。
