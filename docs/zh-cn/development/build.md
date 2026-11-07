# 构建与测试

本机构建和验证：

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

矩阵脚本使用 `CGO_ENABLED=0`，目标为 Windows/Linux/macOS 的 amd64/arm64，输出到 `dist/<os>-<arch>/`。两个程序分别构建。Race 测试需要 CGO 和 C 编译器：`CGO_ENABLED=1 go test -race ./...`。
