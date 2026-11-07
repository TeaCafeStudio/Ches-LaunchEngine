# Go SDK

支援的嵌入介面是 Go 套件 `pkg/sdk`，模組路徑為 `github.com/TeaCafeStudio/Ches-LaunchEngine`。

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

`NewEngine` 回傳穩定的 `core.MinecraftEngine` 介面；新增的 `NewDebugEngine` 回傳支援工作、事件與處理程序診斷的 `core.DebugEngine`。SDK 選項包括遊戲／版本目錄、Java 路徑、離線下載模式、下載並行／重試／逾時、啟動逾時、Logger、HTTP Client 和清單 URL。

目前只實作 Vanilla／離線啟動。外部開發者不應匯入 `internal` 套件；請使用公開的 `pkg/core`、`pkg/models` 和 `pkg/sdk`。
