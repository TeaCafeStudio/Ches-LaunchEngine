# 安裝與建置

## 從原始碼建置

安裝 Go 1.21 或更新版本，並在儲存庫根目錄執行：

```sh
go build -o cle ./cmd/cle
go build -o cle-debug ./cmd/cle-debug
```

Windows：

```powershell
go build -o cle.exe .\cmd\cle
go build -o cle-debug.exe .\cmd\cle-debug
```

自動化多平台建置請參閱[建置文件](development/build.md)。指令碼使用 `CGO_ENABLED=0`，產物寫入 `dist/<os>-<arch>/`。

## Java

安裝與 Minecraft 版本相容的 Java。Minecraft 1.21.1 需要 Java 21。可在啟動要求中設定 `javaPath`，或在 CLE 設定中設定 `java.path`。`cle-debug` 支援 `java detect` 和 `java list`。

## 執行階段相依性

程式是 Go 二進位檔，執行時不需要 Python、Node.js、.NET 或系統 Shell。啟動 Minecraft 仍需要相容的 Java。
