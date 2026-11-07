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

## Game is running but needs to close

Find its PID with `process list`, then run `stop <pid>`. A launch task ID is not
a PID. Use `stop <pid> --force` only after a graceful request is unsuitable.

## Offline profile cannot join a server

Offline profiles are not authenticated Microsoft/Mojang accounts and cannot
join online-mode servers. Online account providers have not been implemented.

## Terminal is not interactive

Piped stdin uses one command per line and exits on EOF. Use
`cle-debug --command "help"` to execute one command without an interactive TTY.
