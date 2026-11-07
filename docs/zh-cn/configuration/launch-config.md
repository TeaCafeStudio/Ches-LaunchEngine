# 启动请求 JSON

启动请求描述一次 Minecraft 启动，不是 CLE 运行配置，也不使用 CLE 配置 Schema 校验。

```json
{
  "versionId": "1.21.1",
  "gameDirectory": "./game",
  "javaPath": "java",
  "account": {"name": "OfflinePlayer", "type": "offline"},
  "loader": {"type": "vanilla"},
  "argumentMode": "auto",
  "window": {"width": 1280, "height": 720}
}
```

| 字段 | 类型 | 必填／默认 | 说明 |
|---|---|---|---|
| `versionId` | string | 引擎必填 | Minecraft 版本 ID |
| `versionDirectory` | string | 可选 | 版本元数据/JAR 目录；否则由游戏目录或配置推导 |
| `gameDirectory` | string | 若配置未提供则必填 | 游戏数据根目录 |
| `javaPath` | string | 可选 | Java 程序；默认使用配置值或自动检测 |
| `javaArguments` | string array | 可选 | 附加 JVM 参数 |
| `gameArguments` | string array | 可选 | 附加游戏参数 |
| `account` | object | 启动需要离线账户 | 内联离线身份或已保存的 `id` |
| `loader` | object | 可选；默认 Vanilla | 目前 `type` 只支持 `vanilla`；`version` 为后续加载器预留 |
| `downloadOptions` | object | 可选 | `offline` 布尔值；`concurrency` 为 0/默认或 1–64；`retries` 为 0/默认或 0–10 |
| `window` | object | 可选 | `width`、`height` 整数和 `fullscreen` 布尔值 |
| `environment` | string map | 可选 | 子进程环境变量覆盖 |
| `workingDirectory` | string | 可选 | 必须是现存目录；否则使用游戏目录 |
| `argumentMode` | string | 可选；默认 `auto` | `auto`、`legacy` 或 `modern` |

账户模型还接受 `id`、`name`、`uuid`、`displayName`、`profile`、`type`、`credentialRef`、`xuid`、`userType` 和时间戳。本阶段只支持离线身份：使用有效 `name` 和可选标准 UUID，或已保存账户 `id`。启动请求不能包含 Token 或密码。

使用已保存账户的例子：

```json
{
  "versionId": "1.21.1",
  "gameDirectory": "C:\\Games\\Minecraft",
  "javaPath": "C:\\Program Files\\Java\\jdk-21\\bin\\java.exe",
  "account": {"id": "replace-with-id-from-account-list"},
  "loader": {"type": "vanilla"}
}
```

Linux/macOS 的 `gameDirectory` 示例：`/home/alex/.minecraft` 或 `/Users/alex/Library/Application Support/minecraft`。如果报错 `unknown field "accountId"`，请使用实际字段 `account.id`。解码器拒绝未知字段和多个 JSON 值。
