# Game processes

`process list` and `process inspect <pid>` report only Minecraft processes
started and tracked by this engine. The PID is the operating-system process ID,
not the CLE launch task ID.

Request a graceful stop:

```text
stop 1234
```

Use force only when explicitly needed:

```text
stop 1234 --force
```

On Unix-like systems, graceful stop sends an interrupt signal. Windows uses a
close request to the Java process's top-level window; if no window is available,
the request may fail and a deliberate force stop may be required.

`process inspect` and `process list` show status and exit code when known.
`logs --process 1234` filters recent events by process. The terminal does not
control unrelated Java processes.
