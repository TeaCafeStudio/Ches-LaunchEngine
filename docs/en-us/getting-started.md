# Getting started

CLE is a cross-platform Go backend that resolves Minecraft metadata, downloads
and verifies Vanilla files, builds launch arguments, and manages the Java
process. It is not a graphical launcher. Use `cle` for formal one-shot CLI
operations and `cle-debug` for interactive diagnostics, live events, tasks,
and process controls.

## Requirements

- Minecraft Java Edition files are downloaded by CLE as needed.
- Java must be installed. Minecraft 1.21.1 requires Java 21.
- For source builds, install Go 1.21 or newer.

## Build and run

From the repository root, build both programs with `go build ./cmd/cle` and
`go build ./cmd/cle-debug`. For release output, use
[`scripts/build.ps1`](../../scripts/build.ps1) on Windows or
[`scripts/build.sh`](../../scripts/build.sh) on Linux/macOS.

Check the executable version and Java runtime:

```powershell
.\cle.exe -version
```

```sh
./cle -version
```

The current formal CLI does not have a Java query command; use `cle-debug
--command "java detect"` to inspect Java.

## Create an offline account

Start `cle-debug` and add a local profile:

```text
account add offline Alex_123
account list
```

An offline profile is not a Mojang/Microsoft identity and cannot authenticate
to online-mode servers. Profiles are stored without tokens in the OS user
configuration directory. See [account configuration](configuration/accounts.md).

## Launch Minecraft

Copy [`examples/launch/vanilla.json`](../../examples/launch/vanilla.json), set
`gameDirectory` and `javaPath` for the machine, then run:

```powershell
.\cle.exe -launch .\examples\launch\vanilla.json
```

```sh
./cle -launch ./examples/launch/vanilla.json
```

To select a stored account, set its `id` in the request's `account` object.
The Debug Terminal can also use its selected account when the request omits
account identity:

```sh
./cle-debug --config ./examples/launch/vanilla.json --command "launch"
```

## Platform path examples

Windows request paths commonly look like
`C:\\Games\\Minecraft` and `C:\\Program Files\\Java\\jdk-21\\bin\\java.exe`.
Linux paths may look like `/home/alex/.minecraft` and
`/usr/lib/jvm/java-21-openjdk/bin/java`. macOS paths may look like
`/Users/alex/Library/Application Support/minecraft` and
`/Library/Java/JavaVirtualMachines/temurin-21.jdk/Contents/Home/bin/java`.
These are examples; install paths vary by machine.

For progress and game logs, use `cle-debug`, then `watch`, `logs`, `tasks`,
`process list`, and `stop <pid>`. See [troubleshooting](debug/troubleshooting.md).
