# 帳戶資料儲存

目前階段只實作離線帳戶 Provider。除錯終端機命令包括 `account add offline <name> [uuid]`、`account list`、`account inspect <id>`、`account profile <id>`、`account validate <id>`、`account switch <id>` 和 `account remove <id>`。

帳戶資料以 JSON 儲存在作業系統使用者設定目錄的 `ChesLaunchEngine/accounts.json`。可透過 CLE 設定的 `account.storePath` 指定其他位置。POSIX 系統會限制目錄和檔案權限為目前使用者；Windows 使用使用者設定檔繼承的 ACL。

此儲存只包含非敏感離線資料，不是加密憑證保管庫；本階段沒有線上 Token 或密碼。非空的 `account.credentialStore` 和線上 `account.defaultProvider` 目前不支援。

預設離線 UUID 使用 Minecraft 的 `OfflinePlayer:<name>` UUID 演算法。自訂 UUID 必須符合標準 8-4-4-4-12 十六進位格式。離線身分無法通過正版線上模式伺服器驗證；由於沒有遠端工作階段，更新和登出操作不支援。
