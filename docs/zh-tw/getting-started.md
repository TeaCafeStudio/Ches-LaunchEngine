# 快速入門

CLE 是跨平台 Minecraft Java Edition 後端：解析版本中繼資料、下載並驗證 Vanilla 檔案、建置啟動參數並管理 Java 處理程序。它不是圖形介面啟動器。`cle` 用於正式的一次性命令；`cle-debug` 提供互動式診斷、即時事件、工作和處理程序管理。

## 環境需求

- CLE 可視需要下載 Minecraft Java Edition 檔案。
- 必須安裝 Java；Minecraft 1.21.1 需要 Java 21。
- 從原始碼建置需要 Go 1.21 或更新版本。

## 建置與執行

在專案根目錄執行 `go build ./cmd/cle` 和 `go build ./cmd/cle-debug`。發行建置可使用 Windows 的 [`scripts/build.ps1`](../../scripts/build.ps1)，或 Linux/macOS 的 [`scripts/build.sh`](../../scripts/build.sh)。

```powershell
.\cle.exe -version
```

```sh
./cle -version
```

正式 CLI 目前沒有 Java 查詢命令；可使用 `cle-debug --command "java detect"` 偵測 Java。

## 建立離線帳戶

啟動 `cle-debug` 後輸入：

```text
account add offline Alex_123
account list
```

離線身分不是 Mojang/Microsoft 帳戶，無法通過正版線上模式伺服器驗證。帳戶資料儲存在作業系統使用者設定目錄中，不含 Token。詳見[帳戶說明](configuration/accounts.md)。

## 啟動 Minecraft

複製 [`examples/launch/vanilla.json`](../../examples/launch/vanilla.json)，設定適合本機的 `gameDirectory` 和 `javaPath`：

```powershell
.\cle.exe -launch .\examples\launch\vanilla.json
```

```sh
./cle -launch ./examples/launch/vanilla.json
```

若要使用已儲存帳戶，請在要求中的 `account` 物件設定 `id`。除錯終端機也可使用目前選取的帳戶：

```sh
./cle-debug --config ./examples/launch/vanilla.json --command "launch"
```

## 路徑範例

Windows：`C:\\Games\\Minecraft`、`C:\\Program Files\\Java\\jdk-21\\bin\\java.exe`。Linux：`/home/alex/.minecraft`、`/usr/lib/jvm/java-21-openjdk/bin/java`。macOS：`/Users/alex/Library/Application Support/minecraft`、`/Library/Java/JavaVirtualMachines/temurin-21.jdk/Contents/Home/bin/java`。實際安裝位置依電腦而異。

若要查看進度和遊戲記錄，請使用 `cle-debug` 的 `watch`、`logs`、`tasks`、`process list` 和 `stop <pid>`。另請參閱[疑難排解](debug/troubleshooting.md)。
