# ChesLaunchEngine
[![GitHub Stars](https://img.shields.io/github/stars/TeaCafeStudio/Ches-LaunchEngine?style=flat-square)](https://github.com/TeaCafeStudio/Ches-LaunchEngine/stargazers)
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
- Offline, third-party Yggdrasil, and Microsoft (premium) authentication.
  Offline and Yggdrasil/Microsoft credentials are stored in the
  operating-system keyring; mod loaders and the HTTP server/API are not
  implemented. Offline profiles cannot authenticate to online-mode servers.
- Microsoft accounts use the OAuth 2.0 Device Code flow, Xbox Live, XSTS,
  Minecraft Services login, entitlement check, and game profile retrieval. The
  Microsoft chain requires a Microsoft public-client application ID configured
  through `account.microsoft.clientId` or the `CLE_MICROSOFT_CLIENT_ID`
  environment variable; CLE ships no hard-coded Client ID or Client Secret.
- Yggdrasil launch injects the pinned Authlib Injector 1.2.8 agent, downloaded
  from its official HTTPS distribution endpoint and SHA-256 verified. Its
  compatibility depends on the Minecraft/Authlib version and Yggdrasil server.
- Yggdrasil and Microsoft protocols, account resolution, and JVM argument
  wiring have automated mock-server coverage; real authentication, entry into
  the Minecraft main menu, and OS credential-store runtime behavior have not
  been verified.

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

Start the developer terminal and add an offline, Yggdrasil, or Microsoft
account:

```text
cle-debug
account add offline Player_Name
```

Microsoft accounts use the OAuth Device Code flow and require a Microsoft
public-client application ID (see
[accounts](docs/en-us/configuration/accounts.md)):

```text
account add microsoft
```

Yggdrasil account passwords are read only through a hidden interactive prompt.
Use the server's Yggdrasil API base URL (the same base used for
`/authserver/authenticate`), not a login webpage:

```text
account add yggdrasil https://auth.example.org login-name
```

Yggdrasil services must use HTTPS. HTTP is accepted only for explicitly
enabled loopback test servers in programmatic use, not by the CLI.
The first Yggdrasil launch downloads Authlib Injector 1.2.8 into the game
directory's verified library cache; offline launch requires that verified JAR
to already exist there. No live Yggdrasil account or service is bundled.

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
