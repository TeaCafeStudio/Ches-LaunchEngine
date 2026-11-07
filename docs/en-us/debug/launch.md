# Launching from the Debug Terminal

Set a default request when starting the program:

```sh
cle-debug --config ./launch.json
```

Then run:

```text
launch
```

Or specify the request in the terminal:

```text
launch --config ./examples/launch/vanilla.json
```

The terminal prints the launch task ID, version, selected Java path, account
type, game directory, loader, current stage, and PID when available. It then
renders event lines such as:

```text
[task_status task=cle-... INFO] Launch task status changed to downloading
[download_progress task=cle-... INFO] Downloaded ... (12/100 files)
[process_started task=cle-... pid=1234 INFO] Minecraft process started
[game_stdout task=cle-... pid=1234 INFO] ...
```

Use `watch <task-id>` or `logs --task <task-id>` to focus output. `debug
arguments <task-id>` displays resolved arguments after sensitive values have
been redacted. Do not copy raw launch credentials into JSON request files.

## Stored offline accounts

Create one with `account add offline Alex`, inspect it with `account list`, and
start a request containing `{"account":{"id":"<stored-id>"}}`. If the request
does not specify account identity, the active offline profile is used. Existing
inline `{"account":{"name":"Alex","type":"offline"}}` requests continue to
work.

Offline profiles are local identities only; they do not prove ownership or
provide online multiplayer authentication.
