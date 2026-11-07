# Launch request JSON

A launch request describes one Minecraft launch. It is not the CLE runtime
configuration and is not checked by the CLE config schema.

```json
{
  "versionId": "1.21.1",
  "gameDirectory": "./game",
  "javaPath": "java",
  "account": {
    "name": "OfflinePlayer",
    "type": "offline"
  },
  "loader": {"type": "vanilla"},
  "argumentMode": "auto",
  "window": {"width": 1280, "height": 720}
}
```

## Fields

| Field | Type | Required / default | Meaning |
|---|---|---|---|
| `versionId` | string | Required by engine | Minecraft version ID. |
| `versionDirectory` | string | Optional | Version metadata/JAR directory; otherwise derived from game/config paths. |
| `gameDirectory` | string | Required unless configured | Game data root. |
| `javaPath` | string | Optional | Java executable; defaults to configured/detected Java. |
| `javaArguments` | string array | Optional | Additional JVM arguments. |
| `gameArguments` | string array | Optional | Additional game arguments. |
| `account` | object | Offline required for launch | Inline offline identity or stored `id`; see below. |
| `loader` | object | Optional; Vanilla | `type` currently supports `vanilla` only; `version` is reserved for loaders. |
| `downloadOptions` | object | Optional | `offline` bool, `concurrency` 0/default or 1-64, `retries` 0/default or 0-10. |
| `window` | object | Optional | `width`, `height` integers and `fullscreen` bool. |
| `environment` | string map | Optional | Child-process environment overrides. |
| `workingDirectory` | string | Optional | Existing directory; otherwise game directory. |
| `argumentMode` | string | Optional; `auto` | `auto`, `legacy`, or `modern`. |

Account fields accepted by the model include `id`, `name`, `uuid`,
`displayName`, `profile`, `type`, `credentialRef`, `xuid`, `userType`, and
timestamps. Phase 4.1.1 supports offline identities only. Use either a valid
offline `name` and optional standard UUID, or a stored account `id`. Tokens and
passwords are not part of a launch request.

## Stored account example

```json
{
  "versionId": "1.21.1",
  "gameDirectory": "C:\\Games\\Minecraft",
  "javaPath": "C:\\Program Files\\Java\\jdk-21\\bin\\java.exe",
  "account": {"id": "replace-with-id-from-account-list"},
  "loader": {"type": "vanilla"}
}
```

For Linux/macOS, for example use `"/home/alex/.minecraft"` or
`"/Users/alex/Library/Application Support/minecraft"` for `gameDirectory`.

An error such as `unknown field "accountId"` means use the actual nested
`account.id` field. The request decoder rejects unknown fields and multiple
JSON values.
