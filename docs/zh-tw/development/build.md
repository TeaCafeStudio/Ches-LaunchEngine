# 建置與測試

本機建置與驗證：

```sh
go build -o cle ./cmd/cle
go build -o cle-debug ./cmd/cle-debug
gofmt -w cmd internal pkg
go test ./...
go vet ./...
```

Windows：

```powershell
go build -o cle.exe .\cmd\cle
go build -o cle-debug.exe .\cmd\cle-debug
.\scripts\build.ps1 all
```

POSIX：

```sh
./scripts/build.sh all
./scripts/build.sh cle
./scripts/build.sh cle-debug
```

矩陣指令碼使用 `CGO_ENABLED=0`，目標為 Windows/Linux/macOS 的 amd64/arm64，輸出到 `dist/<os>-<arch>/`。兩個程式分開建置。Race 測試需要 CGO 和 C 編譯器：`CGO_ENABLED=1 go test -race ./...`。
