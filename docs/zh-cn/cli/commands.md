# CLI 命令参考

以下参数由 `cmd/cle` 解析，不使用位置子命令。

## `cle -version`

用途：显示程序版本。无必填参数。普通输出为版本文本；添加 `--json` 时返回 JSON 响应。

```text
$ cle -version
ChesLaunchEngine dev
```

## `cle -check-config <file>`

用途：读取并校验 CLE 配置 JSON；`<file>` 必填。成功显示 `configuration is valid`；失败返回非零退出码。JSON 模式返回结构化错误。

## `cle -config <file>`

用途：指定本次运行使用的 CLE 配置。可省略，省略时使用内置默认值；可与 `-launch` 或 `-check-config` 组合。单独指定配置不会启动服务。

```text
cle -config cle.json -launch launch.json
```

配置不存在或无效时返回非零退出码；JSON 模式下错误响应写入 stdout，诊断信息写入 stderr。

## `cle -launch <request.json>`

用途：读取启动请求并启动 Minecraft；请求文件必填，CLE 配置可通过 `-config <cle.json>` 指定。程序启动并管理 Java 进程，等待其退出。Ctrl+C 会取消启动任务或请求正在运行的游戏进程停止。

```powershell
cle -config .\cle.json -launch .\examples\launch\vanilla.json
```

成功时显示任务 ID 和 PID；JSON 模式返回最终任务对象。失败时返回非零退出码及错误。

## `cle --json`

为支持的一次性命令输出单个 JSON 响应；stdout 不混入普通日志，诊断写入 stderr。成功退出码为 0，失败为非零。

```text
cle --json -version
cle --json -check-config cle.json
cle --json -launch request.json
```

## `cle account`

账户管理子命令。可用 `--config <cle.json>` 指定 CLE 配置（用于读取
`account.storePath` 和 `account.microsoft.clientId`），并可用 `--json` 输出
JSON。子命令：

| 命令 | 用途 |
|---|---|
| `cle account list` | 列出已保存账户及活动账户标记 |
| `cle account add microsoft` | 启动 OAuth Device Code 登录；验证网址和用户代码写入 stderr，轮询直至完成 |
| `cle account show <account-id>` | 查看非敏感账户资料 |
| `cle account remove <account-id>` | 删除账户及对应凭据 |

```powershell
cle account add microsoft --config .\cle.json
cle account list
cle account show <account-id>
cle account remove <account-id>
```

Microsoft 账户需要 Microsoft 公共客户端应用 ID（配置文件
`account.microsoft.clientId` 或环境变量 `CLE_MICROSOFT_CLIENT_ID`）；CLE 不
内置 Client ID。

## `cle debug`

仅用于迁移提示，不会初始化或导入调试台。请改用 `cle-debug`。
