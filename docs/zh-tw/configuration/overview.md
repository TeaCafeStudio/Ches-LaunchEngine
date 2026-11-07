# 設定概覽

CLE 使用兩種不同的 JSON 文件：

1. CLE 執行設定：由 `-config` 或 `cle-debug --config` 讀取。
2. 單次啟動要求：由 `-launch` 或除錯終端機的 `launch --config` 讀取。

兩者不可互換。CLE 設定會依據 [`configs/cle.schema.json`](../../../configs/cle.schema.json) 和執行階段規則驗證；啟動要求會解碼為 `pkg/models.LaunchRequest`，並拒絕未知欄位。請參閱[啟動要求格式](launch-config.md)。

最小 CLE 設定：

```json
{"version":1}
```

可使用 `cle -check-config cle.json` 驗證。預設值由 `internal/config.Default` 提供；欄位詳見[Schema 與預設值](schema.md)。
