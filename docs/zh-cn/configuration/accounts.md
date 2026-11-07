# 账户与凭据

CLE 支持本地离线资料、第三方 Yggdrasil 账户和 Microsoft 正版账户。Yggdrasil
Provider 与启动集成已实现，并由模拟服务器测试；Microsoft 账户认证链路同样已
实现并由模拟服务器测试。两者的真实认证与实机进入 Minecraft 世界尚未验证。

## Microsoft 正版账户

CLE 通过 OAuth 2.0 Device Code 授权流程接入 Microsoft 账户，完成 Xbox Live、
XSTS、Minecraft Services 登录、Minecraft: Java Edition 授权检查和 Game Profile
获取。该链路已由模拟服务器自动化测试覆盖；真实 Microsoft 认证与实机进入游戏
尚未验证。

### 应用注册与 Client ID

CLE 不内置任何 Client ID，也不会借用其他启动器的 Client ID 或 Client Secret。
运营方需要在 Microsoft Entra 管理中心注册自己的**公共客户端**应用：

- 账户类型选择“任何组织目录中的账户和个人 Microsoft 账户”（consumers）。
- 启用“允许公共客户端流”（Allow public client flows），无需客户端机密。
- Device Code 流程不使用浏览器回调，因此无需配置重定向 URI。

注册完成后，通过以下任一方式把应用程序（客户端）ID 提供给 CLE（优先级从高到
低）：

1. 配置文件：`account.microsoft.clientId`。
2. 环境变量：`CLE_MICROSOFT_CLIENT_ID`。
3. 程序化：`accounts.MicrosoftOptions{ClientID: ...}`。

未配置 Client ID 时，`account add microsoft` 会返回明确错误。

### Device Code 登录流程

1. CLE 向 Microsoft 请求 Device Code。
2. 显示验证网址和用户代码。
3. 用户在浏览器打开网址并输入代码完成授权。
4. CLE 按官方轮询间隔轮询 Token，正确处理 `authorization_pending`、
   `slow_down`、`expired_token`、`authorization_declined` 与网络错误，并支持
   通过上下文取消停止轮询。

### 认证链路

Microsoft Access Token → Xbox Live 用户认证（Xbox User Token + User Hash）→
XSTS 授权（XSTS Token + User Hash）→ Minecraft Services 登录（Minecraft Access
Token）→ Minecraft: Java Edition 授权检查 → Game Profile 获取（UUID、用户名、
Skin、Cape）。

CLE 严格区分 Microsoft Access Token、Xbox User Token、XSTS Token、Minecraft
Access Token 与 Refresh Token，绝不混存到含义不明确的字段。

### Token 与凭据存储

OAuth Refresh Token、OAuth Access Token 和 Minecraft Access Token 通过操作系统
凭据库按账户 ID 隔离保存，不写入账户 JSON 或普通日志。账户 JSON 仅保存非敏感
资料（UUID、用户名、XUID 等）。Minecraft Access Token 有效期较短，CLE 会在
过期时使用 Refresh Token 自动刷新并重跑链路；Refresh Token 不会传给 Minecraft。

### 正版启动参数

Microsoft 正版账户直接把 Minecraft Access Token 传给游戏（`auth_access_token`、
`user_type=msa`、`auth_xuid`），并保持 UUID 与用户名一致；不使用第三方
Yggdrasil 的 Authlib Injector 认证重定向。Skin/Cape 由游戏通过 Mojang 会话
服务器根据 Access Token 自行获取，CLE 不伪造签名 textures。

### 限制

- 真实 Microsoft 认证、凭据库实机运行和实际进入游戏主菜单尚未验证。
- 授权检查仅依据 Entitlement 服务返回的 `game_minecraft`/`product_minecraft`
  项目判断，不跳过检查；没有 Java Edition 授权的账户会得到明确错误提示。

## 第三方 Yggdrasil 启动

`account add yggdrasil` 的 `auth-server-url` 必须是用于
`/authserver/authenticate` 的 Yggdrasil API 基础 URL，而不是网站或登录页面，
并且必须使用 HTTPS。CLE 会向该 Provider 验证会话，将最新 Game Profile 交给
参数构建器，并通过 JVM `-javaagent` 加载 Authlib Injector，使 Minecraft 客户端
也将会话和 Profile 请求发送到该服务。仅传入 `auth_access_token` 参数不会重定向
游戏客户端的 Session Server。

CLE 固定使用 Authlib Injector 1.2.8（build 56），仅从官方 HTTPS 下载地址获取，
并在原子安装到游戏目录的
`libraries/org/authlib-injector/1.2.8` 缓存前校验固定 SHA-256。下载会拒绝重定向。
离线启动要求缓存中已有校验通过的 JAR。该 Agent 使用 AGPL-3.0-or-later，并附带
上游 Authlib Injector 例外条款；详见[上游许可](https://github.com/yushijinhun/authlib-injector/blob/develop/AGPLv3-with-authlib-injector-exception.txt)。

Authlib Injector 会修改 Minecraft 运行时的 Authlib 行为，兼容性会因 Minecraft/
Authlib 版本及 Yggdrasil 服务端 API 功能而异。CLE 不保证所有版本或服务商均可用。
当前自动化测试覆盖 JVM 参数位置和模拟服务器集成，但未使用真实服务或实机游戏。
请查看[上游项目文档](https://github.com/yushijinhun/authlib-injector)了解兼容性和
服务端要求。

## 资料与凭据存储

非敏感账户资料保存在操作系统用户配置目录下的
`ChesLaunchEngine/accounts.json`。可通过 `account.storePath` 指定其他路径。
POSIX 系统会将权限限制为当前用户；Windows 使用用户配置目录继承的 ACL。

Yggdrasil Access Token 和 Client Token 通过操作系统凭据库单独保存，不写入
账户 JSON 或普通日志。Windows 使用 Credential Manager，macOS 使用 Keychain，
Linux 使用 Secret Service。Linux 桌面会话必须提供可用的 D-Bus Secret
Service。凭据库不可用时，添加或使用账户会明确失败；CLE 不会回退到明文
存储或自行设计的加密方案。
平台后端由 `go-keyring` 选择；本阶段尚未在 Windows、macOS 或 Linux 实机验证
凭据库运行行为。交叉编译本身不能证明目标系统凭据服务可用。

`account.credentialStore` 可省略或设为 `os`。密码只通过 `cle-debug` 的隐藏
交互提示输入，不会作为命令行参数。

## 调试台命令

- `account add offline <name> [uuid]` 创建本地资料。UUID 可省略；自定义 UUID
  必须为标准 8-4-4-4-12 十六进制格式。离线账户不能通过正版在线模式服务器认证。
- `account add yggdrasil <auth-server-url> <username>` 会隐藏提示输入密码。
  服务地址必须是 Yggdrasil API 基础 URL 并使用 HTTPS。仅程序化测试可显式启用
  回环地址的 HTTP。
- `account add microsoft` 启动 OAuth Device Code 登录：显示验证网址和用户代码，
  轮询直至授权成功、失败、过期或取消。需要配置 Client ID（见上文）。
- `account list`、`account inspect <id>` 和 `account profile <id>` 显示非敏感资料。
- `account validate <id>` 在本地校验离线资料，或校验 Yggdrasil / Microsoft 会话。
- `account refresh <id>` 刷新 Yggdrasil 或 Microsoft 会话（Microsoft 会重跑完整
  认证链路以获取新的 Minecraft Access Token）。
- `account logout <id>` 使在线会话失效并删除已存凭据；账户资料保留，但需重新
  添加凭据后才能启动。
- `account switch <id>` 选择活动资料；`account remove <id>` 删除资料及其凭据。

在线账户操作需要能够访问所配置的认证服务器及可用的操作系统凭据库。
自动化测试不会使用真实账户。
