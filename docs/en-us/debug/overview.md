# Debug Terminal

`cle-debug` is a separate developer executable. It uses the same engine through
the public `core.DebugEngine` and SDK factory as the formal CLI; it does not
contain a second launcher. The main `cle` program does not import the terminal.

Run interactively:

```sh
cle-debug
cle-debug --config ./cle.json
cle-debug --config ./launch.json
```

`--config` accepts a CLE configuration or a launch request; a request becomes
the default for `launch`. `--launch-config` explicitly sets the default launch
request. Run one command without a prompt using
`cle-debug --config ./launch.json --command "help"`.
For `--command "launch"`, the process remains attached and streams events until
the launch task exits, fails, or is cancelled; Ctrl+C cancels the active task.

The terminal subscribes to bounded Core events, distinguishing engine logs,
game stdout/stderr, download progress, task states, and process lifecycle.
`watch` controls live output, while `logs` queries recent in-memory events.
Sensitive argument values are redacted.

Use `help` inside the terminal. See [commands](commands.md),
[launch](launch.md), [tasks](tasks.md), [processes](processes.md), and
[troubleshooting](troubleshooting.md).
