# CLI command reference

The options below are parsed by `cmd/cle`. There are no positional subcommands.
Use options before any positional arguments.

## `cle -version`

- Purpose: print the binary version.
- Required arguments: none.
- Output: human-readable version, or a JSON `version` response with `--json`.

```text
$ cle -version
ChesLaunchEngine dev
```

## `cle -check-config <file>`

- Purpose: load and validate a CLE configuration JSON file.
- Required: `<file>`.
- Success: `configuration is valid`; JSON mode returns that message in `data`.
- Failure: non-zero exit; JSON mode returns a structured error.

```text
$ cle -check-config cle.json
configuration is valid
```

## `cle -config <file>`

- Purpose: load the CLE runtime configuration used by this invocation.
- Required: no; defaults apply when omitted.
- Default: built-in defaults from `internal/config`.
- Relationship: may be combined with `-launch` or `-check-config`.
- A config path alone prints usage; it does not start a server.
- Example: `cle -config cle.json -launch launch.json`.
- Failure example: a missing/invalid config exits non-zero; with `--json`,
  stderr remains diagnostic-only and stdout contains the error envelope.

## `cle -launch <request.json>`

- Purpose: resolve and launch Minecraft using a launch-request JSON file.
- Required: `<request.json>`.
- Configuration: optional `-config <cle.json>`.
- Behavior: starts the managed Java process and waits for it to exit; Ctrl+C
  cancels a pre-launch task or asks a running game process to stop.
- Success: prints the task ID and PID; JSON mode returns the final task object.
- Failure: non-zero exit and an error on stderr, or a JSON error response.

```powershell
cle -config .\cle.json -launch .\examples\launch\vanilla.json
```

## `cle --json`

- Purpose: emit exactly one JSON response to stdout for supported one-shot
  commands.
- Required: no; when supplied without a command, returns a help-style result.
- Diagnostics: stderr; normal log console output is disabled in JSON mode.
- Exit code: `0` on success; non-zero on failure.
- Supported combinations include `cle --json -version`,
  `cle --json -check-config cle.json`, and
  `cle --json -launch request.json`.

## `cle account`

Account management subcommands. Use `--config <cle.json>` to load
`account.storePath` and `account.microsoft.clientId`, and `--json` for a JSON
response.

| Command | Purpose |
|---|---|
| `cle account list` | List stored accounts and mark the active account |
| `cle account add microsoft` | Start the OAuth Device Code login; the verification URL and user code are written to stderr, then the command polls until completion |
| `cle account show <account-id>` | Show non-secret account metadata |
| `cle account remove <account-id>` | Remove the account and its stored credentials |

```powershell
cle account add microsoft --config .\cle.json
cle account list
cle account show <account-id>
cle account remove <account-id>
```

Microsoft accounts require a Microsoft public-client application ID
(`account.microsoft.clientId` in the configuration file or the
`CLE_MICROSOFT_CLIENT_ID` environment variable); CLE ships no hard-coded
Client ID.

## `cle debug`

Deprecated migration path only. It does not initialize or import the terminal.
Use `cle-debug` instead.
