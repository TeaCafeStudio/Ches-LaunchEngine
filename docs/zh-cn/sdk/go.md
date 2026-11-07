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

目前只实现 Vanilla／离线启动。外部开发者不应导入 `internal` 包；请使用公开的 `pkg/core`、`pkg/models` 和 `pkg/sdk`。
