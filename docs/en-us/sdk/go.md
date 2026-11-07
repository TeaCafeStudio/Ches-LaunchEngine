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
client, manifest URL, and an optional public `core.AccountManager`. Supply the
account manager when launching stored accounts:

```go
engine, err := sdk.NewEngine(sdk.Options{
    GameDirectory: ".minecraft",
    Accounts:      accountManager,
})
```

The launch request should contain only `models.Account{ID: storedAccountID}`;
Core resolves the selected provider, validates/refreshes the session, and
builds the profile arguments. For config-based construction use
`NewEngineWithConfigAndAccounts` or `NewDebugEngineWithConfigAndAccounts`.

Vanilla/offline launch and Yggdrasil launch are implemented. The Yggdrasil
path injects Authlib Injector 1.2.8 as a JVM agent after verifying its pinned
SHA-256. Protocol and launch wiring have mock coverage, but real authentication
and entering the Minecraft main menu have not been verified. Microsoft
authentication, mod loaders, and HTTP API are not implemented. External
developers should not import `internal` packages; use public `pkg/core`,
`pkg/models`, and `pkg/sdk`.
