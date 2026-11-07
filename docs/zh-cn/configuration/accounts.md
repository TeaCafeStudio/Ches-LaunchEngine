# 账户资料存储

当前阶段只实现离线账户 Provider。调试台命令包括 `account add offline <name> [uuid]`、`account list`、`account inspect <id>`、`account profile <id>`、`account validate <id>`、`account switch <id>` 和 `account remove <id>`。

账户资料以 JSON 保存在操作系统用户配置目录的 `ChesLaunchEngine/accounts.json`。可通过 CLE 配置的 `account.storePath` 指定其他位置。POSIX 系统会限制目录和文件权限为当前用户；Windows 使用用户配置目录继承的 ACL。

此存储仅含非敏感离线资料，不是加密凭据库；本阶段没有在线 Token 或密码。非空 `account.credentialStore` 和在线 `account.defaultProvider` 暂不支持。

默认离线 UUID 使用 Minecraft 的 `OfflinePlayer:<name>` UUID 算法。自定义 UUID 必须符合标准 8-4-4-4-12 十六进制格式。离线身份无法通过正版在线模式服务器认证；由于没有远程会话，刷新和注销操作不支持。
