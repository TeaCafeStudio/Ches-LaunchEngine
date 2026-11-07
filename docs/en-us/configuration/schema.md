# CLE configuration schema

The authoritative JSON Schema is [`configs/cle.schema.json`](../../../configs/cle.schema.json).
The Go decoder also applies defaults and runtime validation. Root-level
`version` is required and must be `1`; unknown fields and trailing JSON values
are errors.

## Fields and defaults

| JSON path | Type | Required | Default / constraints |
|---|---|---|---|
| `version` | integer | Yes | Exactly `1`. |
| `paths.gameDirectory` | string | No | Empty; can be supplied in request. |
| `paths.versionsDirectory` | string | No | Empty; derived from game directory. |
| `launch.argumentMode` | string | No | `auto`; allowed `auto`, `legacy`, `modern`. |
| `launch.timeoutSeconds` | integer | No | `3600`; positive. |
| `download.concurrency` | integer | No | `8`; range 1-64. |
| `download.retries` | integer | No | `3`; range 0-10. |
| `download.timeoutSeconds` | integer | No | `300`; positive. |
| `download.offline` | boolean | No | `false`. |
| `java.path` | string | No | Empty; detect from host. |
| `account.defaultProvider` | string | No | Empty; only `offline` is currently accepted. |
| `account.credentialStore` | string | No | Empty; non-empty value is rejected in Phase 4.1.1. |
| `account.storePath` | string | No | Empty; OS user config path is used. Must not be whitespace. |
| `logging.level` | string | No | `INFO`; `DEBUG`, `INFO`, `WARN`, `ERROR`. |
| `logging.console` | boolean | No | `true`; logs go to stderr. |
| `logging.directory` | string | No | Empty; if set, `instanceId` is required. |
| `logging.instanceId` | string | Conditional | Required with log directory. |
| `api.enabled` | boolean | No | `false`; HTTP API is not implemented. |
| `api.address` | string | No | `127.0.0.1:8765`; required if API enabled, but no server is started. |
| `debugTerminal.enabled` | boolean | No | `true`. |
| `debugTerminal.historyLimit` | integer | No | `1000`; range 0-10000. |

`cle -check-config <file>` validates a CLE configuration. It does not validate
a launch request. The schema's defaults are descriptive; the Go `Default`
function supplies runtime values.

## Minimal, complete and invalid examples

Minimal:

```json
{"version":1}
```

Complete:

```json
{
  "version": 1,
  "paths": {"gameDirectory": "./game", "versionsDirectory": "./game/versions"},
  "launch": {"argumentMode": "auto", "timeoutSeconds": 3600},
  "download": {"concurrency": 8, "retries": 3, "timeoutSeconds": 300, "offline": false},
  "java": {"path": "java"},
  "account": {"defaultProvider": "offline", "storePath": "./accounts.json"},
  "logging": {"level": "INFO", "console": true},
  "api": {"enabled": false, "address": "127.0.0.1:8765"},
  "debugTerminal": {"enabled": true, "historyLimit": 1000}
}
```

Windows paths use JSON escaping, for example `"C:\\Games\\CLE"`. Linux/macOS
can use `"~"` only if the caller expands it; CLE does not promise shell
expansion. An invalid example is `{"version":1,"download":{"concurrency":0}}`;
use 1-64.
