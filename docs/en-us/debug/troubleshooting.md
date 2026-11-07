# Troubleshooting

## Java not found or incompatible

Run `java detect` or `java list`. Set `javaPath` in the launch request or
`java.path` in CLE configuration. Minecraft 1.21.1 needs Java 21.

## Metadata or downloads fail

Check network access, disk space, the version ID, and the configured game and
version directories. `download status` shows active task download counts;
`download list` displays recent progress events.

## Launch task failed

Run `tasks`, then `status <task-id>`. Inspect the reported error code and failed
stage. Use `logs --task <task-id>` and `debug info <task-id>`. The engine
redacts recognized token/password assignments in rendered log text.

For `ACCOUNT_NOT_FOUND`, check that the launch request uses the stored account
ID shown by `account list`. For `SESSION_INVALID` or
`SESSION_REFRESH_FAILED`, check the configured Yggdrasil API base URL and OS
credential-store availability; do not put tokens or passwords in the request.
For `AUTHLIB_INJECTOR_UNAVAILABLE` or a download/integrity error, retry with
network access enabled. Offline mode requires the verified Authlib Injector
1.2.8 JAR in the game's library cache. Compatibility also depends on the
Minecraft/Authlib version and provider.

## Game is running but needs to close

Find its PID with `process list`, then run `stop <pid>`. A launch task ID is not
a PID. Use `stop <pid> --force` only after a graceful request is unsuitable.

## Offline profile cannot join a server

Offline profiles are not authenticated Microsoft/Mojang accounts and cannot
join online-mode servers. Yggdrasil support is implemented but has not been
verified against a real service or live Minecraft launch. Microsoft
authentication is not implemented.

## Terminal is not interactive

Piped stdin uses one command per line and exits on EOF. Use
`cle-debug --command "help"` to execute one command without an interactive TTY.
