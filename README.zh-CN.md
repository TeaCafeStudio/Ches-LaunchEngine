# ChesLaunchEngine
[![GitHub Stars](https://img.shields.io/github/stars/TeaCafeStudio/Ches-LaunchEngine?style=flat-square)](https://github.com/TeaCafeStudio/Ches-LaunchEngine/stargazers)
[![GitHub Forks](https://img.shields.io/github/forks/TeaCafeStudio/Ches-LaunchEngine?style=flat-square)](https://github.com/TeaCafeStudio/Ches-LaunchEngine/network/members)
[![GitHub Issues](https://img.shields.io/github/issues/TeaCafeStudio/Ches-LaunchEngine?style=flat-square)](https://github.com/TeaCafeStudio/Ches-LaunchEngine/issues)
[![GitHub Releases](https://img.shields.io/github/v/release/TeaCafeStudio/Ches-LaunchEngine?style=flat-square)](https://github.com/TeaCafeStudio/Ches-LaunchEngine/releases)

> **语言：** [English](README.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md)

ChesLaunchEngine（CLE）是 Minecraft: Java Edition 的模块化、跨平台 Go 后端。
它是一个引擎，而非图形界面启动器。

## 程序

- `cle`：正式的一次性命令行界面，用于版本显示、配置校验和 Minecraft 启动。
- `cle-debug`：独立的开发者终端，用于交互式命令、账户资料管理、实时启动
  事件、日志、任务和进程。
- `pkg/sdk`：基于共享引擎的 Go 嵌入 API。

这两个可执行文件使用相同的 SDK/Core 引擎。`cmd/cle` 不依赖终端包。旧的
`cle debug` 形式会打印迁移提示；请改用 `cle-debug`。

## 当前支持

- Vanilla 启动和离线资料。
- 版本元数据、Java 运行时检测、经校验的库/资源、启动参数构建、受管进程以及
  Debug Terminal 事件输出。
- 离线、第三方 Yggdrasil 和 Microsoft（正版）认证。离线账户以及
  Yggdrasil/Microsoft 凭据存储在操作系统凭据库中；Mod 加载器和 HTTP
  服务器/API 尚未实现。离线资料无法通过在线模式服务器认证。
- Microsoft 账户使用 OAuth 2.0 Device Code 流程、Xbox Live、XSTS、Minecraft
  Services 登录、授权检查以及游戏资料获取。Microsoft 认证链需要通过
  `account.microsoft.clientId` 或 `CLE_MICROSOFT_CLIENT_ID` 环境变量配置的
  Microsoft 公共客户端应用 ID；CLE 不内置任何 Client ID 或 Client Secret。
- Yggdrasil 启动会注入固定版本的 Authlib Injector 1.2.8 代理，从官方 HTTPS
  分发端点下载并经过 SHA-256 校验。其兼容性取决于 Minecraft/Authlib 版本及
  Yggdrasil 服务器。
- Yggdrasil 和 Microsoft 协议、账户解析以及 JVM 参数接线具有自动化模拟服务器
  测试覆盖；真实认证、进入 Minecraft 主菜单以及操作系统凭据库运行时行为尚未
  验证。

## 快速开始

从仓库根目录构建两个程序：

```powershell
go build -o cle.exe .\cmd\cle
go build -o cle-debug.exe .\cmd\cle-debug
```

```sh
go build -o cle ./cmd/cle
go build -o cle-debug ./cmd/cle-debug
```

检查版本并校验 CLE 配置：

```text
cle -version
cle -check-config cle.json
```

编辑 [`examples/launch/vanilla.json`](examples/launch/vanilla.json)，使其匹配
本机的 Java/游戏路径，然后启动：

```text
cle -launch ./examples/launch/vanilla.json
```

启动开发者终端并添加离线、Yggdrasil 或 Microsoft 账户：

```text
cle-debug
account add offline Player_Name
```

Microsoft 账户使用 OAuth Device Code 流程，需要 Microsoft 公共客户端应用 ID
（参见[账户说明](docs/zh-cn/configuration/accounts.md)）：

```text
account add microsoft
```

Yggdrasil 账户密码仅通过隐藏交互提示读取。请使用服务器的 Yggdrasil API 基础
URL（即 `/authserver/authenticate` 所用的基础地址），而不是登录网页：

```text
account add yggdrasil https://auth.example.org login-name
```

Yggdrasil 服务必须使用 HTTPS。HTTP 仅允许用于程序化使用中显式启用的回环测试
服务器，CLI 不允许。首次 Yggdrasil 启动会将 Authlib Injector 1.2.8 下载到
游戏目录中经校验的库缓存；离线启动要求该经校验的 JAR 已存在。不捆绑任何真实
Yggdrasil 账户或服务。

有关终端详情，请运行 `help` 或参阅
[Debug Terminal 命令指南](docs/zh-cn/debug/commands.md)。

## CLI JSON 输出

受支持的一次性 CLI 命令接受 `--json`；stdout 包含一个 JSON 响应，诊断信息
保留在 stderr：

```text
cle --json -version
cle --json -check-config cle.json
cle --json -config cle.json -launch launch.json
```

响应结构和错误码行为请参阅 [CLI JSON 输出](docs/zh-cn/cli/json-output.md)。

## 配置与文档

开发者文档提供 [简体中文](docs/zh-cn/README.md)、
[English](docs/en-us/README.md) 和 [繁體中文](docs/zh-tw/README.md) 版本。

CLE 运行时配置由 [`configs/cle.schema.json`](configs/cle.schema.json) 定义。
启动请求是单独的 JSON 文档，由 `pkg/models.LaunchRequest` 表示。

[开发者文档索引](docs/README.md) 链接了快速开始、安装、当前所有 CLI 与 Debug
Terminal 命令、启动/配置指南、JSON 输出、Go SDK 以及构建/贡献说明。HTTP API
明确标注为 **计划中 / 尚未实现**；设置 `api.enabled` 不会启动服务器。

## 构建与校验

```powershell
go test ./...
go vet ./...
.\scripts\build.ps1 all
```

```sh
go test ./...
go vet ./...
./scripts/build.sh all
```

构建脚本会为 Windows、Linux、macOS 的 amd64 与 arm64 架构生成独立的 `cle` 和
`cle-debug` 二进制文件，并禁用 CGO。参见
[开发/构建](docs/zh-cn/development/build.md)。
