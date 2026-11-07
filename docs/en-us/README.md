# CLE developer documentation (English)

Languages: [简体中文](../zh-cn/README.md) · [English](README.md) · [繁體中文](../zh-tw/README.md)

ChesLaunchEngine (CLE) is a Go engine for Minecraft: Java Edition. It provides
the formal `cle` CLI, the separate `cle-debug` developer terminal, and a Go SDK.

## Documentation map

- [Getting started](getting-started.md) · [Installation](installation.md)
- [CLI overview](cli/overview.md) · [Commands](cli/commands.md) · [JSON output](cli/json-output.md)
- [Debug overview](debug/overview.md) · [Commands](debug/commands.md) · [Launch](debug/launch.md) · [Tasks](debug/tasks.md) · [Processes](debug/processes.md) · [Troubleshooting](debug/troubleshooting.md)
- [Configuration](configuration/overview.md) · [Launch request](configuration/launch-config.md) · [Accounts](configuration/accounts.md) · [Schema](configuration/schema.md)
- [HTTP API status](api/overview.md) · [JSON conventions](api/json-format.md) · [Planned endpoints](api/endpoints.md)
- [Go SDK](sdk/go.md) · [Build](development/build.md) · [Architecture](development/architecture.md) · [Contributing](development/contribution.md)

Offline and Yggdrasil account support are implemented. Yggdrasil launch uses a
SHA-256-pinned Authlib Injector 1.2.8 Java Agent; real authentication, reaching
the Minecraft main menu, and OS credential-store runtime behavior have not
been verified. Microsoft authentication, non-Go SDK bindings, mod loaders,
and the HTTP API are not implemented.
