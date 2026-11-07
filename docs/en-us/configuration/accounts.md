# Accounts and credentials

CLE supports local Offline profiles, third-party Yggdrasil accounts, and
Microsoft (premium) accounts. The Yggdrasil and Microsoft providers and their
launch integration are implemented and covered by mock-server tests. Real
authentication, reaching the Minecraft main menu, and OS credential-store
runtime behavior have not been verified.

## Microsoft (premium) accounts

CLE uses the OAuth 2.0 Device Code flow for Microsoft accounts, followed by
Xbox Live authentication, XSTS authorization, Minecraft Services login, the
Minecraft: Java Edition entitlement check, and Game Profile retrieval. This
chain has mock-server test coverage; real Microsoft authentication and a live
game have not been verified.

### Application registration and Client ID

CLE ships no hard-coded Client ID and never borrows another launcher's Client
ID or Client Secret. Operators must register their own **public client**
application in the Microsoft Entra admin center:

- Select the account type that includes personal Microsoft accounts (consumers).
- Enable "Allow public client flows"; no client secret is required.
- No redirect URI is required for the Device Code flow.

Provide the application (client) ID through any of the following (highest
priority first):

1. Configuration file: `account.microsoft.clientId`.
2. Environment variable: `CLE_MICROSOFT_CLIENT_ID`.
3. Programmatically: `accounts.MicrosoftOptions{ClientID: ...}`.

Without a Client ID, `account add microsoft` returns a clear error.

### Device Code login flow

1. CLE requests a Device Code from Microsoft.
2. The verification URL and user code are displayed.
3. The user opens the URL in a browser and enters the code.
4. CLE polls the token endpoint at the documented interval, handling
   `authorization_pending`, `slow_down`, `expired_token`,
   `authorization_declined`, and network errors, and supports cancellation.

### Authentication chain

Microsoft Access Token → Xbox Live user authentication (Xbox User Token +
User Hash) → XSTS authorization (XSTS Token + User Hash) → Minecraft Services
login (Minecraft Access Token) → Minecraft: Java Edition entitlement check →
Game Profile retrieval (UUID, username, skin, cape).

CLE keeps the Microsoft Access Token, Xbox User Token, XSTS Token, Minecraft
Access Token, and Refresh Token strictly separated.

### Token and credential storage

The OAuth refresh token, OAuth access token, and Minecraft access token are
stored in the operating-system credential store keyed by account ID, never in
account JSON or ordinary logs. Account JSON stores only non-secret data (UUID,
username, XUID, and so on). Minecraft access tokens are short-lived; CLE
refreshes them with the refresh token when expired. The refresh token is never
passed to Minecraft.

### Premium launch arguments

Microsoft accounts pass the Minecraft access token directly to the game
(`auth_access_token`, `user_type=msa`, `auth_xuid`) with a consistent UUID and
username, and do not use the third-party Yggdrasil Authlib Injector redirect.
Skin/cape data is fetched by the game from the Mojang session server using the
access token; CLE does not fabricate a signed textures property.

### Limitations

- Real Microsoft authentication, credential-store runtime behavior, and
  reaching the Minecraft main menu have not been verified.
- Entitlement is checked only against `game_minecraft`/`product_minecraft`
  items returned by the entitlement service; the check is never skipped, and
  accounts without a Java Edition entitlement receive a clear error.

## Third-party Yggdrasil launch

The `auth-server-url` supplied to `account add yggdrasil` must be the
Yggdrasil API base URL used for `/authserver/authenticate`, not a website or
login page. Use HTTPS. CLE verifies the session through that provider, passes
the current Game Profile to the argument builder, and adds Authlib Injector
as a JVM `-javaagent` so the Minecraft client uses the configured service for
session/profile requests. An `auth_access_token` argument alone does not
redirect the game's session service.

CLE pins Authlib Injector 1.2.8 (build 56), downloaded only from the official
HTTPS artifact endpoint and checked against the pinned SHA-256 digest before
being atomically placed in the game's `libraries/org/authlib-injector/1.2.8`
cache. Redirects are rejected. Offline launches require a valid cached JAR.
The agent is licensed under AGPL-3.0-or-later with the upstream
Authlib-Injector exception; see the [upstream license](https://github.com/yushijinhun/authlib-injector/blob/develop/AGPLv3-with-authlib-injector-exception.txt).

Authlib Injector modifies Minecraft's runtime Authlib behavior, and
compatibility varies by Minecraft/Authlib version and by Yggdrasil server API
features. CLE does not claim that every Minecraft version or provider works.
Only the launch argument placement and mock-server integration are automated
here; no real service or live game has been tested. Check the
[upstream project documentation](https://github.com/yushijinhun/authlib-injector)
for current compatibility and server requirements.

## Profile and credential storage

Non-secret account profile data is stored in
`ChesLaunchEngine/accounts.json` under the OS user configuration directory.
Set `account.storePath` to use another path. POSIX permissions are restricted
to the current user; Windows inherits the user-profile ACL.

Yggdrasil access and client tokens are stored separately through the operating
system credential store, never in the account JSON or ordinary logs. Windows
uses Credential Manager, macOS uses Keychain, and Linux uses Secret Service.
Linux desktop sessions must provide a working D-Bus Secret Service. If the OS
credential store is unavailable, adding or using the account fails; CLE does
not fall back to plaintext or custom encryption.
The platform backends are selected by `go-keyring`; this phase has not
validated runtime credential-store behavior on Windows, macOS, or Linux.
Cross-compilation alone does not establish that the platform service works.

`account.credentialStore` may be omitted or set to `os`. Passwords are accepted
only by the interactive `cle-debug` hidden prompt and are not command-line
arguments.

## Debug Terminal commands

- `account add offline <name> [uuid]` creates a local profile. UUID is optional;
  a supplied UUID must be canonical 8-4-4-4-12 hexadecimal. CLE does not claim
  Offline profiles can authenticate to online-mode servers.
- `account add yggdrasil <auth-server-url> <username>` prompts for the
  password without echo. The server URL must be the Yggdrasil API base and use
  HTTPS. HTTP is available only for explicitly enabled loopback test servers
  in programmatic use.
- `account add microsoft` starts the OAuth Device Code login: it shows the
  verification URL and user code and polls until success, failure, expiry, or
  cancellation. A Client ID must be configured (see above).
- `account list`, `account inspect <id>`, and `account profile <id>` show
  non-secret profile information.
- `account validate <id>` validates an Offline profile locally or validates a
  Yggdrasil/Microsoft session.
- `account refresh <id>` refreshes a Yggdrasil or Microsoft session (Microsoft
  re-runs the full chain to obtain a new Minecraft access token).
- `account logout <id>` invalidates and deletes stored online credentials;
  the account profile remains, but cannot launch until credentials are added
  again.
- `account switch <id>` selects the active profile; `account remove <id>`
  removes the profile and its stored credentials.

Online account operations require access to the configured authentication
server and a working OS credential store. No real account is included in
automated tests.
