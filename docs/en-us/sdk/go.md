# Go SDK

The supported embedding API is the Go package `pkg/sdk`. The module is
`github.com/TeaCafeStudio/Ches-LaunchEngine`.

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

`NewEngine` returns the stable `core.MinecraftEngine` interface. The additive
`NewDebugEngine` returns `core.DebugEngine` for task/event/process diagnostics.
SDK options include game/version directories, Java path, offline download
mode, download concurrency/retries/timeouts, launch timeout, logger, HTTP
client, and manifest URL.

Only Vanilla/offline launch is implemented. External developers should not
import `internal` packages; use public `pkg/core`, `pkg/models`, and `pkg/sdk`.
