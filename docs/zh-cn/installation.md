# 安装与构建

## 从源码构建

安装 Go 1.21 或更新版本，在仓库根目录执行：

```sh
go build -o cle ./cmd/cle
go build -o cle-debug ./cmd/cle-debug
```

Windows：

```powershell
go build -o cle.exe .\cmd\cle
go build -o cle-debug.exe .\cmd\cle-debug
```

自动化多平台构建见[构建文档](development/build.md)。脚本使用 `CGO_ENABLED=0`，产物写入 `dist/<os>-<arch>/`。

## Java

安装与 Minecraft 版本兼容的 Java。Minecraft 1.21.1 需要 Java 21。可在启动请求中设置 `javaPath`，或在 CLE 配置中设置 `java.path`。`cle-debug` 支持 `java detect` 和 `java list`。

## 运行时依赖

程序是 Go 二进制文件，运行时不需要 Python、Node.js、.NET 或系统 Shell。启动 Minecraft 仍需要兼容的 Java。
