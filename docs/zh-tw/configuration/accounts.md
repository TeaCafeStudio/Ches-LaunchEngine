# 帳戶與憑證

CLE 支援本機離線資料、第三方 Yggdrasil 帳戶和 Microsoft 正版帳戶。Yggdrasil
與 Microsoft Provider 及其啟動整合皆已實作，並由模擬伺服器測試；兩者的真實
驗證與實機進入 Minecraft 世界尚未驗證。

## Microsoft 正版帳戶

CLE 透過 OAuth 2.0 Device Code 授權流程接入 Microsoft 帳戶，完成 Xbox Live、
XSTS、Minecraft Services 登入、Minecraft: Java Edition 授權檢查與 Game Profile
取得。此鏈路已由模擬伺服器自動化測試覆蓋；真實 Microsoft 驗證與實機進入遊戲
尚未驗證。

### 應用程式註冊與 Client ID

CLE 不內建任何 Client ID，也不會借用其他啟動器的 Client ID 或 Client Secret。
營運方需要在 Microsoft Entra 管理中心註冊自己的**公開用戶端**應用程式：

- 帳戶類型選擇包含個人 Microsoft 帳戶的選項（consumers）。
- 啟用「允許公開用戶端流程」（Allow public client flows），無需用戶端機密。
- Device Code 流程不使用瀏覽器回呼，因此無需設定重新導向 URI。

註冊完成後，透過以下任一方式將應用程式（用戶端）ID 提供給 CLE（優先順序由高
至低）：

1. 設定檔：`account.microsoft.clientId`。
2. 環境變數：`CLE_MICROSOFT_CLIENT_ID`。
3. 程式化：`accounts.MicrosoftOptions{ClientID: ...}`。

未設定 Client ID 時，`account add microsoft` 會傳回明確錯誤。

### Device Code 登入流程

1. CLE 向 Microsoft 要求 Device Code。
2. 顯示驗證網址與使用者代碼。
3. 使用者在瀏覽器開啟網址並輸入代碼完成授權。
4. CLE 依官方輪詢間隔輪詢 Token，正確處理 `authorization_pending`、
   `slow_down`、`expired_token`、`authorization_declined` 與網路錯誤，並支援取消。

### 驗證鏈路

Microsoft Access Token → Xbox Live 使用者驗證（Xbox User Token + User Hash）→
XSTS 授權（XSTS Token + User Hash）→ Minecraft Services 登入（Minecraft Access
Token）→ Minecraft: Java Edition 授權檢查 → Game Profile 取得（UUID、使用者
名稱、Skin、Cape）。

CLE 嚴格區分 Microsoft Access Token、Xbox User Token、XSTS Token、Minecraft
Access Token 與 Refresh Token，絕不混存。

### Token 與憑證儲存

OAuth Refresh Token、OAuth Access Token 和 Minecraft Access Token 透過作業系統
憑證庫按帳戶 ID 隔離儲存，不寫入帳戶 JSON 或一般記錄。帳戶 JSON 僅儲存非敏感
資料（UUID、使用者名稱、XUID 等）。Minecraft Access Token 有效期較短，CLE 會在
過期時使用 Refresh Token 自動重新整理並重跑鏈路；Refresh Token 不會傳給
Minecraft。

### 正版啟動參數

Microsoft 正版帳戶直接將 Minecraft Access Token 傳給遊戲（`auth_access_token`、
`user_type=msa`、`auth_xuid`），並保持 UUID 與使用者名稱一致；不使用第三方
Yggdrasil 的 Authlib Injector 驗證重新導向。Skin/Cape 由遊戲透過 Mojang 工作
階段伺服器依 Access Token 自行取得，CLE 不偽造簽章 textures。

### 限制

- 真實 Microsoft 驗證、憑證庫實機執行與實際進入遊戲主選單尚未驗證。
- 授權檢查僅依 Entitlement 服務回傳的 `game_minecraft`/`product_minecraft`
  項目判斷，不跳過檢查；沒有 Java Edition 授權的帳戶會得到明確錯誤提示。

## 第三方 Yggdrasil 啟動

`account add yggdrasil` 的 `auth-server-url` 必須是用於
`/authserver/authenticate` 的 Yggdrasil API 基礎網址，而不是網站或登入頁面，
並且必須使用 HTTPS。CLE 會向該 Provider 驗證工作階段，將最新 Game Profile
交給參數建置器，並透過 JVM `-javaagent` 載入 Authlib Injector，使 Minecraft
用戶端也將工作階段和 Profile 要求傳送至該服務。只傳入 `auth_access_token`
參數不會重新導向遊戲用戶端的 Session Server。

CLE 固定使用 Authlib Injector 1.2.8（build 56），僅從官方 HTTPS 下載網址取得，
並在原子安裝至遊戲目錄的
`libraries/org/authlib-injector/1.2.8` 快取前驗證固定 SHA-256。下載會拒絕重新導向。
離線啟動要求快取中已有通過驗證的 JAR。該 Agent 採用 AGPL-3.0-or-later，並附帶
上游 Authlib Injector 例外條款；詳見[上游授權](https://github.com/yushijinhun/authlib-injector/blob/develop/AGPLv3-with-authlib-injector-exception.txt)。

Authlib Injector 會修改 Minecraft 執行階段的 Authlib 行為，相容性會因 Minecraft/
Authlib 版本及 Yggdrasil 伺服器 API 功能而異。CLE 不保證所有版本或服務商均可用。
目前自動化測試涵蓋 JVM 參數位置和模擬伺服器整合，但未使用真實服務或實機遊戲。
請參閱[上游專案文件](https://github.com/yushijinhun/authlib-injector)了解相容性和
伺服器需求。

## 資料與憑證儲存

非敏感帳戶資料儲存在作業系統使用者設定目錄下的
`ChesLaunchEngine/accounts.json`。可透過 `account.storePath` 指定其他路徑。
POSIX 系統會將權限限制為目前使用者；Windows 使用使用者設定檔繼承的 ACL。

Yggdrasil Access Token 和 Client Token 透過作業系統憑證庫分開儲存，不會寫入
帳戶 JSON 或一般記錄。Windows 使用 Credential Manager，macOS 使用 Keychain，
Linux 使用 Secret Service。Linux 桌面工作階段必須提供可用的 D-Bus Secret
Service。憑證庫無法使用時，新增或使用帳戶會明確失敗；CLE 不會回退至明文
儲存或自行設計的加密方案。
平台後端由 `go-keyring` 選擇；本階段尚未在 Windows、macOS 或 Linux 實機驗證
憑證庫執行行為。交叉編譯本身不能證明目標系統憑證服務可用。

`account.credentialStore` 可省略或設為 `os`。密碼只透過 `cle-debug` 的隱藏
互動提示輸入，不會作為命令列參數。

## 除錯終端機命令

- `account add offline <name> [uuid]` 建立本機資料。UUID 可省略；自訂 UUID
  必須是標準 8-4-4-4-12 十六進位格式。離線帳戶無法通過正版線上模式伺服器驗證。
- `account add yggdrasil <auth-server-url> <username>` 會以隱藏提示輸入密碼。
  服務網址必須是 Yggdrasil API 基礎網址並使用 HTTPS。只有程式化測試可明確啟用
  迴路位址的 HTTP。
- `account add microsoft` 啟動 OAuth Device Code 登入：顯示驗證網址與使用者
  代碼，並輪詢至成功、失敗、過期或取消。需先設定 Client ID（見上文）。
- `account list`、`account inspect <id>` 和 `account profile <id>` 顯示非敏感資料。
- `account validate <id>` 在本機驗證離線資料，或驗證 Yggdrasil / Microsoft 工作階段。
- `account refresh <id>` 更新 Yggdrasil 或 Microsoft 工作階段（Microsoft 會重跑
  完整鏈路以取得新的 Minecraft Access Token）。
- `account logout <id>` 使線上工作階段失效並刪除已儲存憑證；帳戶資料仍保留，
  但必須重新新增憑證後才能啟動。
- `account switch <id>` 選擇作用中資料；`account remove <id>` 刪除資料及其憑證。

線上帳戶操作需要連線至設定的驗證伺服器及可用的作業系統憑證庫。
自動化測試不會使用真實帳戶。
