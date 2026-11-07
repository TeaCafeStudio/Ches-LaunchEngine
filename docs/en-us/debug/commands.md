# Debug Terminal command reference

Commands are parsed by `internal/terminal`; command names and argument counts
are validated by each handler. Examples below show representative output.

## Basics

| Input | Purpose / arguments | Representative output |
|---|---|---|
| `help` | List commands; no arguments. | `Commands:` followed by command usage. |
| `clear` | Clear interactive screen; no arguments. In pipe mode emits a blank line. | Clears the interactive screen. |
| `version` | Print CLE version; no arguments. | `ChesLaunchEngine dev` |
| `version list` | Display up to 25 manifest versions. | `1.21.1 release ...` |
| `version inspect 1.21.1` | Resolve and display metadata. | `ID: 1.21.1`, type, main class, required Java major, assets, library count. |
| `version resolve 1.21.1` | Resolve metadata and report success. | Version summary plus `Version metadata resolved successfully.` |
| `exit` / `quit` | Leave terminal; no arguments. Active engine resources are closed. | Returns to the calling shell. |

## Launch tasks and processes

| Command | Purpose / arguments | Example |
|---|---|---|
| `launch [--config <request.json>]` | Start the default or specified request asynchronously. | `Launch Task ID: cle-...`, version, Java path, account type, stage, pending PID. |
| `tasks` | List managed launch tasks; no arguments. | `<task-id> version=1.21.1 stage=running pid=1234 exit=pending` or `No launch tasks.` |
| `status [task-id]` | Show one task, or all tasks if ID omitted. | `<task-id> version=1.21.1 stage=running pid=1234 exit=pending` |
| `cancel <task-id>` | Cancel a launch task, primarily during resolution/download. | On success no text; task status changes to cancelled. |
| `process [list]` | List managed Minecraft processes. | `pid=1234 task=cle-... version=1.21.1 status=running exit=pending` |
| `process inspect <pid>` | Inspect one managed process. PID must be positive. | Same process summary, or a not-found error. |
| `stop <pid> [--force]` | Gracefully stop a game process; force termination must be explicit. | `Graceful stop requested for process 1234.` |

## Versions, Java, downloads, logs

| Input | Purpose | Representative output |
|---|---|---|
| `java list` | List detected Java runtimes. | `Java 21... (major 21, amd64): <path>` or `No usable Java runtime was found.` |
| `java detect` | Detect a runtime using configured/default search. | `Java 21... (major 21, amd64): <path>` |
| `java inspect C:\Java\bin\java.exe` | Inspect a specified executable. | Java version, major, architecture, and path. |
| `download status` | Show counts for tasks currently downloading. | `task=cle-... downloads=12/100 stage=downloading` |
| `download list` | Show recent download progress events. | `[download_progress task=cle-...] Downloaded ...` |
| `download cancel <task-id>` | Cancel the download stage of that launch task. | On success no text; task cancellation follows. |
| `logs --task <id> --limit 20` | Display recent matching events; default limit is 100, max 2000. | `[game_stdout task=cle-... pid=1234 INFO] ...` |
| `logs --process <pid>` | Filter recent events by process. | Matching stdout/stderr and lifecycle events. |
| `logs --task <id> --follow` | Display matching history and enable live watching. | `Following matching events; use unwatch to stop.` |
| `watch` | Display new events from all tasks. | `Watching events from all tasks; use unwatch to stop.` |
| `watch <task-id>` | Filter live events by task. | `Watching events for <task-id>; use unwatch to stop.` |
| `watch <pid>` | Filter live events by managed process. | Same message using the PID. |
| `unwatch` | Stop displaying live events. | `Stopped displaying watched events.` |

## Debug details and accounts

| Input | Purpose | Representative output |
|---|---|---|
| `debug config` | Print loaded CLE config. | Indented JSON config. |
| `debug info <task-id>` | Show launch stage, Java, PID, and download counts. | `task=... version=1.21.1 java=... stage=running pid=1234 downloads=...` |
| `debug arguments <task-id>` | Show Java executable and redacted JVM/Game arguments. | `Java executable: ...`, `JVM arguments:`, `Main class: ...`, working directory. |
| `debug classpath <task-id>` | Show classpath entries. | `Classpath entries: 70` followed by paths. |
| `debug environment <task-id>` | Show configured overrides with sensitive values redacted. | Sorted `KEY=value` lines. |
| `account list` | List stored profiles and mark the active profile. | `<uuid> offline Alex (active)` |
| `account add offline Alex [uuid]` | Create and select a local offline profile. UUID is optional, canonical format. | `Created offline account Alex (<id>), UUID <uuid>.` |
| `account add yggdrasil <auth-server-url> <username>` | Add a Yggdrasil account; password is read through a hidden interactive prompt, never from an argument. Requires HTTPS. | `Created Yggdrasil account Alex (<id>), UUID <uuid>.` |
| `account add microsoft` | Start the OAuth Device Code login; shows the verification URL and user code and polls until completion. Requires a configured Client ID. | `Created Microsoft account Alex (<id>), UUID <uuid>.` |
| `account inspect <id>` | Display non-secret account metadata. | ID, type, username, display name, UUID, creation/update timestamps. |
| `account profile <id>` | Display profile ID/name and property count. | `Profile: Alex (<uuid>), properties: 0` |
| `account validate <id>` | Validate an offline profile locally or a Yggdrasil session remotely. | `Account profile is valid.` |
| `account switch <id>` | Select an offline profile as active. | `Active account set to <id>` |
| `account refresh <id>` | Refresh a Yggdrasil session. Offline refresh is unsupported. | `Account refreshed: <id>` |
| `account logout <id>` | Invalidate and delete Yggdrasil credentials; profile remains. Offline logout is unsupported. | `Account session logged out.` |
| `account remove <id>` | Delete the profile and associated Yggdrasil credentials. | `Account removed.` |

Yggdrasil tokens use the operating-system credential store (Windows Credential
Manager, macOS Keychain, or Linux Secret Service); Linux requires an available
Secret Service session. Microsoft accounts use the OAuth Device Code flow; their
refresh token, OAuth access token, and Minecraft access token are also stored in
the operating-system credential store keyed by account ID. Microsoft accounts
require a Microsoft public-client application ID (`account.microsoft.clientId` or
the `CLE_MICROSOFT_CLIENT_ID` environment variable); CLE ships no hard-coded
Client ID.

`cle-debug --command "help"` executes one command and exits; a one-shot
`launch` waits for the task to finish and streams its events. This is not a
machine-oriented JSON protocol. Use `cle --json` for supported formal CLI
responses.
