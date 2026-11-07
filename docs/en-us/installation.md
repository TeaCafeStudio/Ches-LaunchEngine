# Installation and builds

## Build from source

Install Go 1.21 or newer, clone or download the source, then run:

```sh
go build -o cle ./cmd/cle
go build -o cle-debug ./cmd/cle-debug
```

On Windows, Go adds `.exe` when the output name includes that extension:

```powershell
go build -o cle.exe .\cmd\cle
go build -o cle-debug.exe .\cmd\cle-debug
```

Automated matrix builds are documented in [development/build.md](development/build.md).
They set `CGO_ENABLED=0` and write artifacts under `dist/<os>-<arch>/`.

## Java

Install a Java runtime compatible with the Minecraft version. Minecraft 1.21.1
requires Java 21. Supply `javaPath` in the launch request or set `java.path` in
the CLE configuration. `cle-debug` can report detected runtimes with
`java detect` and `java list`.

## Runtime dependencies

The programs are Go binaries. No Python, Node.js, .NET, or system shell is
required at runtime. A compatible Java runtime is required to run Minecraft.
