# CLI JSON 输出

在正式 CLI 命令中添加 `--json`：

```sh
cle --json -version
cle --json -check-config ./cle.json
cle --json -config ./cle.json -launch ./launch.json
```

CLI 向 stdout 写入一行 JSON：

```json
{"success":true,"data":{"command":"version","version":"dev"},"error":null}
```

`data` 取决于命令。启动响应包含 `{"command":"launch","task":{...}}`，任务字段遵循 `pkg/models.LaunchTask` 的 JSON 标签。

失败响应使用相同结构和稳定错误码：

```json
{"success":false,"data":null,"error":{"code":"INVALID_CONFIGURATION","message":"decode configuration JSON"}}
```

引擎错误使用其定义的错误码；其他错误使用 `CLI_ERROR`。错误消息会脱敏，不包含堆栈、凭据或 Token。调用方应同时检查 `success` 和进程退出码。

JSON 模式不会把控制台日志或人类可读启动消息写入 stdout；已配置的文件日志可以保留。编码诊断写入 stderr。该模式每条命令只返回一次，不提供流式进度；实时任务和下载事件请使用 `cle-debug`。

PowerShell：

```powershell
$response = cle --json -version | ConvertFrom-Json
if (-not $response.success) { throw $response.error.message }
$response.data.version
```

POSIX Shell：

```sh
response="$(cle --json -version)" || exit $?
printf '%s\n' "$response" | jq -r '.data.version'
```

`jq` 只是可选的 JSON 处理工具，不是 CLE 运行时依赖。
