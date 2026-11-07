# 架构

```text
cle CLI -----------------------> pkg/sdk -> core.MinecraftEngine -> engine
cle-debug -> internal/terminal -> pkg/sdk -> core.DebugEngine ----> engine
```

正式 CLI Adapter 位于 `internal/cli`；`cmd/cle` 委托给它，不依赖 `internal/terminal`。独立入口 `cmd/cle-debug` 负责交互式初始化并导入终端。两者使用相同 SDK 工厂和 Vanilla 引擎实现，不存在重复启动器。

`pkg/core` 和 `pkg/models` 定义共享接口与数据；`pkg/sdk` 创建引擎；`internal/config` 负责严格解析 CLE 配置与启动请求；`internal/terminal` 通过公开 Debug Core 接口实现命令和展示。事件来自引擎现有的有界事件总线。

`pkg/accounts` 包含离线和 Yggdrasil Provider 及账户管理器。Yggdrasil 会话 Token 与账户资料 JSON 分开，保存在操作系统凭据库。CLI 和调试台只向 Core 传递已保存账户 ID；Core 在版本安装和进程创建前选择 Provider 并解析有效会话。同一账户的刷新、验证、启动会话解析、注销和删除会串行执行。

Yggdrasil JVM 参数包含来自官方分发源、经过固定 SHA-256 校验的 Authlib Injector 1.2.8 Agent。Minecraft 客户端的会话请求独立于 CLE 登录请求，因此需要此注入。模拟服务器和参数位置测试覆盖该集成，但尚未验证真实认证、实机启动或 Windows/macOS/Linux 凭据库运行行为。Microsoft 认证尚未实现。

目前没有 HTTP API 包或服务端。`api.enabled` 是预留配置，不会启用监听器。
