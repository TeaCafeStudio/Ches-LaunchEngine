# CLI JSON output

Add `--json` to the implemented one-shot CLI options:

```sh
cle --json -version
cle --json -check-config ./cle.json
cle --json -config ./cle.json -launch ./launch.json
```

The CLI writes one response object followed by a newline to stdout:

```json
{"success":true,"data":{"command":"version","version":"dev"},"error":null}
```

The `data` value depends on the command. A launch response contains
`{"command":"launch","task":{...}}`, where the task uses the JSON tags in
`pkg/models.LaunchTask`.

Errors use the same envelope and a stable code:

```json
{"success":false,"data":null,"error":{"code":"INVALID_CONFIGURATION","message":"decode configuration JSON"}}
```

Engine errors use their defined codes. Errors without an engine code use
`CLI_ERROR`. Messages are redacted; cause/stack data and tokens are not
included. Consumers should check both `success` and the process exit code.

In JSON mode, CLE suppresses console logs and human-readable launch messages
from stdout. Configured file logging can remain enabled. Diagnostics for an
encoding failure go to stderr. This is a one-response CLI, not a streaming
progress protocol; use `cle-debug` for download and task events.

Example PowerShell parsing:

```powershell
$response = cle --json -version | ConvertFrom-Json
if (-not $response.success) { throw $response.error.message }
$response.data.version
```

Example POSIX parsing:

```sh
response="$(cle --json -version)" || exit $?
printf '%s\n' "$response" | jq -r '.data.version'
```

`jq` is an optional consumer utility, not a CLE runtime dependency.
