# 架构

```text
cle CLI -----------------------> pkg/sdk -> core.MinecraftEngine -> engine
cle-debug -> internal/terminal -> pkg/sdk -> core.DebugEngine ----> engine
```

正式 CLI Adapter 位于 `internal/cli`；`cmd/cle` 委托给它，不依赖 `internal/terminal`。独立入口 `cmd/cle-debug` 负责交互式初始化并导入终端。两者使用相同 SDK 工厂和 Vanilla 引擎实现，不存在重复启动器。

`pkg/core` 和 `pkg/models` 定义共享接口与数据；`pkg/sdk` 创建引擎；`internal/config` 负责严格解析 CLE 配置与启动请求；`internal/terminal` 通过公开 Debug Core 接口实现命令和展示。事件来自引擎现有的有界事件总线。

目前没有 HTTP API 包或服务端。`api.enabled` 是预留配置，不会启用监听器。Microsoft/Yggdrasil Provider 尚未实现。
