# CLE 配置 Schema

权威 JSON Schema：[`configs/cle.schema.json`](../../../configs/cle.schema.json)。Go 解码器还会应用默认值并执行运行时校验。根级 `version` 必填且必须为 `1`；未知字段和多余 JSON 值会报错。

| JSON 路径 | 类型 | 必填 | 默认值／限制 |
|---|---|---|---|
| `version` | integer | 是 | 必须为 `1` |
| `paths.gameDirectory` | string | 否 | 空；可由启动请求提供 |
| `paths.versionsDirectory` | string | 否 | 空；从游戏目录推导 |
| `launch.argumentMode` | string | 否 | `auto`；允许 `auto`、`legacy`、`modern` |
| `launch.timeoutSeconds` | integer | 否 | `3600`；必须为正数 |
| `download.concurrency` | integer | 否 | `8`；1–64 |
| `download.retries` | integer | 否 | `3`；0–10 |
| `download.timeoutSeconds` | integer | 否 | `300`；必须为正数 |
| `download.offline` | boolean | 否 | `false` |
| `java.path` | string | 否 | 空；自动检测 |
| `account.defaultProvider` | string | 否 | 空；目前只支持 `offline` |
| `account.credentialStore` | string | 否 | 空；非空值当前不支持 |
| `account.storePath` | string | 否 | 空；使用操作系统用户配置路径；不能全为空白 |
| `logging.level` | string | 否 | `INFO`；允许 `DEBUG`、`INFO`、`WARN`、`ERROR` |
| `logging.console` | boolean | 否 | `true`；日志写入 stderr |
| `logging.directory` | string | 否 | 空；设置后须同时提供 `instanceId` |
| `logging.instanceId` | string | 条件必填 | 使用日志目录时必填 |
| `api.enabled` | boolean | 否 | `false`；HTTP API 尚未实现 |
| `api.address` | string | 否 | `127.0.0.1:8765`；启用配置时要求填写，但不会启动服务 |
| `debugTerminal.enabled` | boolean | 否 | `true` |
| `debugTerminal.historyLimit` | integer | 否 | `1000`；0–10000 |

`cle -check-config <file>` 校验 CLE 配置，不校验启动请求。Schema 中的默认值仅作说明；运行默认值由 Go 的 `Default` 函数设置。

最小配置：`{"version":1}`。

完整配置：

```json
{
  "version": 1,
  "paths": {"gameDirectory": "./game", "versionsDirectory": "./game/versions"},
  "launch": {"argumentMode": "auto", "timeoutSeconds": 3600},
  "download": {"concurrency": 8, "retries": 3, "timeoutSeconds": 300, "offline": false},
  "java": {"path": "java"},
  "account": {"defaultProvider": "offline", "storePath": "./accounts.json"},
  "logging": {"level": "INFO", "console": true},
  "api": {"enabled": false, "address": "127.0.0.1:8765"},
  "debugTerminal": {"enabled": true, "historyLimit": 1000}
}
```

Windows 路径须在 JSON 中转义，例如 `"C:\\Games\\CLE"`。Linux/macOS 使用 `~` 时须由调用方展开；CLE 不保证执行 Shell 展开。错误示例 `{"version":1,"download":{"concurrency":0}}` 会被拒绝，并发数须为 1–64。
