# Go SDK

受支持的嵌入接口是 Go 包 `pkg/sdk`，模块路径为 `github.com/TeaCafeStudio/Ches-LaunchEngine`。

```go
engine, err := sdk.NewEngine(sdk.Options{
    GameDirectory: ".minecraft",
    JavaPath:      "java",
})
if err != nil {
    return err
}
defer engine.Close()

request := models.LaunchRequest{
    VersionID:     "1.21.1",
    GameDirectory: ".minecraft",
    Account: models.Account{
        Name: "OfflinePlayer",
        Type: models.AccountOffline,
    },
    Loader: models.Loader{Type: models.LoaderVanilla},
}
if err := engine.Validate(ctx, request); err != nil {
    return err
}
task, err := engine.Launch(ctx, request)
```

`NewEngine` 返回稳定的 `core.MinecraftEngine` 接口；新增的 `NewDebugEngine` 返回支持任务、事件和进程诊断的 `core.DebugEngine`。SDK 选项包括游戏/版本目录、Java 路径、离线下载模式、下载并发/重试/超时、启动超时、Logger、HTTP Client 和清单 URL。

启动已保存账户时，请在 `sdk.Options.Accounts` 提供公开的 `core.AccountManager`，并仅在 `LaunchRequest` 中传入 `models.Account{ID: storedAccountID}`。Core 会选择 Provider 并在进程创建前验证或刷新会话。使用配置构造函数时，可选用 `NewEngineWithConfigAndAccounts` 或 `NewDebugEngineWithConfigAndAccounts`。

目前已实现 Vanilla／离线启动及 Yggdrasil 启动，后者会加载经过 SHA-256 固定校验的 Authlib Injector 1.2.8 Java Agent。协议和参数接线由模拟测试覆盖，但尚未验证真实认证和进入 Minecraft 主菜单。Microsoft 认证、模组加载器和 HTTP API 尚未实现。外部开发者不应导入 `internal` 包；请使用公开的 `pkg/core`、`pkg/models` 和 `pkg/sdk`。
