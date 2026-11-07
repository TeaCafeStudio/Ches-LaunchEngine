# 配置概览

CLE 使用两种不同的 JSON 文件：

1. CLE 运行配置：由 `-config` 或 `cle-debug --config` 读取。
2. 单次启动请求：由 `-launch` 或调试台的 `launch --config` 读取。

两者不能互换。CLE 配置依据 [`configs/cle.schema.json`](../../../configs/cle.schema.json) 和运行时规则校验；启动请求解码为 `pkg/models.LaunchRequest`，并拒绝未知字段。参阅[启动请求格式](launch-config.md)。

最小 CLE 配置：

```json
{"version":1}
```

可用 `cle -check-config cle.json` 校验。默认值由 `internal/config.Default` 提供；字段详见[Schema 与默认值](schema.md)。
