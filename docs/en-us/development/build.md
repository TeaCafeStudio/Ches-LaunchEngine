# Build and test

## Local commands

```sh
go build -o cle ./cmd/cle
go build -o cle-debug ./cmd/cle-debug
gofmt -w cmd internal pkg
go test ./...
go vet ./...
```

Windows:

```powershell
go build -o cle.exe .\cmd\cle
go build -o cle-debug.exe .\cmd\cle-debug
.\scripts\build.ps1 all
```

POSIX:

```sh
./scripts/build.sh all
./scripts/build.sh cle
./scripts/build.sh cle-debug
```

The matrix scripts target Windows/Linux/macOS amd64/arm64 with `CGO_ENABLED=0`
and place binaries under `dist/<os>-<arch>/`. Both programs are separate
builds. To verify just one platform without output files:

```powershell
$env:GOOS="linux"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"
go build ./cmd/cle
go build ./cmd/cle-debug
```

Race tests require CGO and a C compiler: `CGO_ENABLED=1 go test -race ./...`.
