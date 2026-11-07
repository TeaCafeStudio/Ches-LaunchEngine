# ChesLaunchEngine
[![GitHub Stars](https://img.shields.io/github/stars/TeaCafeStudio/Ches-LaunchEngine?style=flat-square)(https://github.com/TeaCafeStudio/Ches-LaunchEngine/stargazers)
[![GitHub Forks](https://img.shields.io/github/forks/Cheeses233/MinecraftParticleImageGenerator?style=flat-square)](https://github.com/TeaCafeStudio/Ches-LaunchEngine/network/members)
[![GitHub Issues](https://img.shields.io/github/issues/Cheeses233/MinecraftParticleImageGenerator?style=flat-square)](https://github.com/TeaCafeStudio/Ches-LaunchEngine/issues)
[![GitHub Releases](https://img.shields.io/github/v/release/TeaCafeStudio/Ches-LaunchEngine?style=flat-square)](https://github.com/TeaCafeStudio/Ches-LaunchEngine/releases)

ChesLaunchEngine (CLE) is a modular, cross-platform Go backend for Minecraft:
Java Edition. It is an engine, not a graphical launcher.

## Programs

- `cle`: formal one-shot CLI for version display, configuration validation,
  and Minecraft launch.
- `cle-debug`: independent developer terminal for interactive commands,
  account profile management, live launch events, logs, tasks, and processes.
- `pkg/sdk`: Go embedding API over the shared engine.

The two executables use the same SDK/Core engine. `cmd/cle` does not depend on
the terminal package. The old `cle debug` form prints a migration message; use
`cle-debug`.

## Current support

- Vanilla launch and offline profiles.
- Version metadata, Java runtime detection, verified libraries/assets, launch
  argument construction, managed processes, and Debug Terminal event output.
- Microsoft and Yggdrasil authentication, mod loaders, and HTTP server/API are
  not implemented. Offline profiles cannot authenticate to online-mode
  servers.

## Quick start

Build both programs from the repository root:

```powershell
go build -o cle.exe .\cmd\cle
go build -o cle-debug.exe .\cmd\cle-debug
```

```sh
go build -o cle ./cmd/cle
go build -o cle-debug ./cmd/cle-debug
```

Check the version and validate CLE configuration:

```text
cle -version
cle -check-config cle.json
```

Edit [`examples/launch/vanilla.json`](examples/launch/vanilla.json) for the
machine's Java/game paths, then launch:

```text
cle -launch ./examples/launch/vanilla.json
```

Start the developer terminal and add an offline profile:

```text
cle-debug
account add offline Player_Name
```

For terminal details, run `help` or see the
[Debug Terminal command guide](docs/en-us/debug/commands.md).

## CLI JSON output

Supported one-shot CLI commands accept `--json`; stdout contains one JSON
response, and diagnostics remain on stderr:

```text
cle --json -version
cle --json -check-config cle.json
cle --json -config cle.json -launch launch.json
```

See [CLI JSON output](docs/en-us/cli/json-output.md) for the response schema and
error-code behavior.

## Configuration and docs

Developer documentation is available in
[简体中文](docs/zh-cn/README.md), [English](docs/en-us/README.md), and
[繁體中文](docs/zh-tw/README.md).

CLE runtime configuration is defined by
[`configs/cle.schema.json`](configs/cle.schema.json). Launch requests are a
separate JSON document represented by `pkg/models.LaunchRequest`.

The [developer documentation index](docs/README.md) links getting started,
installation, all current CLI and Debug Terminal commands, launch/configuration
guides, JSON output, the Go SDK, and build/contribution notes. The HTTP API is
explicitly documented as **Planned / Not Implemented**; setting `api.enabled`
does not start a server.

## Build and validate

```powershell
go test ./...
go vet ./...
.\scripts\build.ps1 all
```

```sh
go test ./...
go vet ./...
./scripts/build.sh all
```

The build scripts produce separate `cle` and `cle-debug` binaries for Windows,
Linux, and macOS on amd64 and arm64, with CGO disabled. See
[development/build](docs/en-us/development/build.md).
