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

## Stored accounts

Create an Offline profile with `account add offline Alex`, or a Yggdrasil
account with `account add yggdrasil <auth-server-url> <username>`. The latter
prompts for the password without echo and requires HTTPS. Inspect accounts
with `account list`, then start a request containing
`{"account":{"id":"<stored-id>"}}`. If the request does not specify account
identity, the active stored profile is used. Inline
`{"account":{"name":"Alex","type":"offline"}}` requests continue to work.

Yggdrasil launches use the account's OS-keyring session; no token belongs in
the launch JSON. The account's Yggdrasil API base URL is passed to Authlib
Injector as a JVM agent so the Minecraft client uses the third-party service
for its own session requests. CLE downloads the pinned, SHA-256-verified
Authlib Injector 1.2.8 artifact on first launch; offline mode needs that
verified artifact already cached under the game directory. Compatibility
depends on the Minecraft/Authlib version and service; no real authentication
or live game launch has been verified. Offline profiles are local identities
only; they do not prove ownership or provide online multiplayer authentication.
