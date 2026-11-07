# 快速开始

CLE 是跨平台 Minecraft Java Edition 后端：解析版本元数据、下载并校验 Vanilla 文件、构造启动参数并管理 Java 进程。它不是图形界面启动器。`cle` 用于正式的一次性命令；`cle-debug` 提供交互式诊断、实时事件、任务和进程管理。

## 环境要求

- CLE 可按需下载 Minecraft Java Edition 文件。
- 需要安装 Java；Minecraft 1.21.1 需要 Java 21。
- 从源码构建需要 Go 1.21 或更新版本。

## 构建和运行

在项目根目录执行 `go build ./cmd/cle` 和 `go build ./cmd/cle-debug`。发布构建可使用 Windows 的 [`scripts/build.ps1`](../../scripts/build.ps1) 或 Linux/macOS 的 [`scripts/build.sh`](../../scripts/build.sh)。

```powershell
.\cle.exe -version
```

```sh
./cle -version
```

正式 CLI 目前没有 Java 查询命令；可用 `cle-debug --command "java detect"` 检测 Java。

## 创建离线账户

启动 `cle-debug` 后输入：

```text
account add offline Alex_123
account list
```

离线身份不是 Mojang/Microsoft 账户，不能通过正版在线模式服务器认证。账户资料保存在操作系统用户配置目录中，不包含 Token。详见[账户说明](configuration/accounts.md)。

## 启动 Minecraft

复制 [`examples/launch/vanilla.json`](../../examples/launch/vanilla.json)，设置适合本机的 `gameDirectory` 和 `javaPath`：

```powershell
.\cle.exe -launch .\examples\launch\vanilla.json
```

```sh
./cle -launch ./examples/launch/vanilla.json
```

要使用已保存账户，在请求中的 `account` 对象设置 `id`。调试台也可使用当前选中的账户：

```sh
./cle-debug --config ./examples/launch/vanilla.json --command "launch"
```

## 路径示例

Windows：`C:\\Games\\Minecraft`、`C:\\Program Files\\Java\\jdk-21\\bin\\java.exe`。Linux：`/home/alex/.minecraft`、`/usr/lib/jvm/java-21-openjdk/bin/java`。macOS：`/Users/alex/Library/Application Support/minecraft`、`/Library/Java/JavaVirtualMachines/temurin-21.jdk/Contents/Home/bin/java`。实际安装位置因机器而异。

查看进度和游戏日志，请使用 `cle-debug` 的 `watch`、`logs`、`tasks`、`process list` 和 `stop <pid>`。另见[故障排除](debug/troubleshooting.md)。
