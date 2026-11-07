# Launch tasks and cancellation

A **launch task** represents the complete resolve/validate/download/prepare/
launch workflow. `tasks` lists these workflow records; `status <task-id>` shows
the current stage and, when available, PID and exit code. Typical stages are
`resolving`, `validating`, `downloading`, `preparing`, `launching`, `running`,
and terminal states such as `failed`, `cancelled`, `stopped`, or `exited`.

`cancel <task-id>` cancels the launch operation, especially while metadata or
files are still being prepared. It is not the command for stopping a running
game. `download cancel <task-id>` targets the task's download stage.

The task ID is CLE's identifier for one launch workflow. It is not an OS
process ID. Use `process list` to see live game processes associated with tasks.

When a task fails, `status <task-id>` reports its error code, message, failed
stage, recent task events, and a likely diagnostic hint. When Minecraft starts
and later exits, the process-exit event and exit code distinguish that from a
pre-launch failure.
